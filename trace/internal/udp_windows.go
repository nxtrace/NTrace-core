//go:build windows && amd64

package internal

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	wd "github.com/xjasonlyu/windivert-go"

	"github.com/nxtrace/NTrace-core/util"
)

type UDPSpec struct {
	IPVersion int
	ICMPMode  int
	SrcIP     net.IP
	DstIP     net.IP
	DstPort   int
	icmp      net.PacketConn
	addr      wd.Address
	handle    wd.Handle
}

func (s *UDPSpec) InitUDP() {
	handle, err := wd.Open("false", wd.LayerNetwork, 0, 0)
	if err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(InitUDP) WinDivert open failed: %v", err))
		}
		log.Fatalf("(InitUDP) WinDivert open failed: %v", err)
	}
	s.handle = handle

	// 设置出站 Address
	s.addr.SetLayer(wd.LayerNetwork)
	s.addr.SetEvent(wd.EventNetworkPacket)
	s.addr.SetOutbound()
}

func (s *UDPSpec) Close() {
	_ = s.icmp.Close()
	_ = s.handle.Close()
}

func (s *UDPSpec) ListenOut(_ context.Context, _ chan struct{}, _ func(srcPort, seq, ttl int, start time.Time)) {
}

// resolveICMPMode 进行最终模式判定
func (s *UDPSpec) resolveICMPMode() int {
	icmpMode := s.ICMPMode
	if icmpMode != 1 && icmpMode != 2 {
		icmpMode = 0 // 统一成 Auto
	}

	// 指定 1=Socket：直接返回
	if icmpMode == 1 {
		return 1
	}

	// Auto(0) 或强制 Sniff(2) → 尝试 WinDivert
	ok, err := winDivertAvailable()
	if !ok {
		if icmpMode == 2 {
			log.Printf("WinDivert sniff mode requested, but WinDivert is not available: %v; falling back to Socket mode.", err)
		}
		return 1
	}
	return 2
}

func (s *UDPSpec) ListenICMP(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, data []byte)) {
	switch s.resolveICMPMode() {
	case 1:
		s.listenICMPSock(ctx, ready, onICMP)
	case 2:
		s.listenICMPWinDivert(ctx, ready, onICMP)
	}
}

func (s *UDPSpec) listenICMPWinDivert(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, data []byte)) {
	// 构造 WinDivert 过滤器：入站 ICMP/ICMPv6，目标为本机 s.SrcIP
	var filter string
	if s.IPVersion == 4 {
		filter = fmt.Sprintf("inbound and icmp and ip.DstAddr == %s", s.SrcIP.String())
	} else {
		filter = fmt.Sprintf("inbound and icmpv6 and ipv6.DstAddr == %s", s.SrcIP.String())
	}

	// 以嗅探模式打开 WinDivert：只复制匹配的包，不拦截
	sniffHandle, err := wd.Open(filter, wd.LayerNetwork, 0, wd.FlagSniff|wd.FlagRecvOnly)
	if err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(ListenICMP) WinDivert open failed: %v (filter=%q)", err, filter))
		}
		log.Fatalf("(ListenICMP) WinDivert open failed: %v (filter=%q)", err, filter)
	}
	defer sniffHandle.Close()

	_ = sniffHandle.SetParam(wd.QueueLength, 8192)
	_ = sniffHandle.SetParam(wd.QueueTime, 4000)

	close(ready)

	buf := make([]byte, 65535)
	var addr wd.Address

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := sniffHandle.Recv(buf, &addr)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			continue
		}

		finish := time.Now()
		raw := make([]byte, n)
		copy(raw, buf[:n])

		var firstLayer gopacket.Decoder
		if s.IPVersion == 4 {
			firstLayer = layers.LayerTypeIPv4
		} else {
			firstLayer = layers.LayerTypeIPv6
		}
		pkt := gopacket.NewPacket(raw, firstLayer, gopacket.NoCopy)

		outer := raw

		var peerIP net.IP
		var data []byte
		if s.IPVersion == 4 {
			ip4, ok := pkt.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
			if !ok || ip4 == nil {
				continue
			}
			peerIP = ip4.SrcIP

			ic4, ok := pkt.Layer(layers.LayerTypeICMPv4).(*layers.ICMPv4)
			if !ok || ic4 == nil {
				continue
			}
			data = ic4.Payload

			switch ic4.TypeCode.Type() {
			case layers.ICMPv4TypeTimeExceeded:
			case layers.ICMPv4TypeDestinationUnreachable:
			default:
				continue
			}

			if len(data) < 20 || data[0]>>4 != 4 {
				continue
			}

			dstIP := net.IP(data[16:20])
			if !dstIP.Equal(s.DstIP) {
				continue
			}
		} else {
			ip6, ok := pkt.Layer(layers.LayerTypeIPv6).(*layers.IPv6)
			if !ok || ip6 == nil {
				continue
			}
			peerIP = ip6.SrcIP

			ic6, ok := pkt.Layer(layers.LayerTypeICMPv6).(*layers.ICMPv6)
			if !ok || ic6 == nil {
				continue
			}
			data = ic6.Payload[4:]

			switch ic6.TypeCode.Type() {
			case layers.ICMPv6TypeTimeExceeded:
			case layers.ICMPv6TypePacketTooBig:
			case layers.ICMPv6TypeDestinationUnreachable:
			default:
				continue
			}

			if len(data) < 40 || data[0]>>4 != 6 {
				continue
			}

			dstIP := net.IP(data[24:40])
			if !dstIP.Equal(s.DstIP) {
				continue
			}
		}
		peer := &net.IPAddr{IP: peerIP}

		msg := ReceivedMessage{
			Peer: peer,
			Msg:  outer,
		}
		onICMP(msg, finish, data)
	}
}

func (s *UDPSpec) SendUDP(ctx context.Context, ipHdr ipLayer, udpHdr *layers.UDP, payload []byte) (time.Time, error) {
	select {
	case <-ctx.Done():
		return time.Time{}, context.Canceled
	default:
	}

	_ = udpHdr.SetNetworkLayerForChecksum(ipHdr)

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	// 序列化 IP 与 UDP 头以及 payload 到缓冲区
	if err := gopacket.SerializeLayers(buf, opts, ipHdr, udpHdr, gopacket.Payload(payload)); err != nil {
		return time.Time{}, err
	}

	start := time.Now()

	// 复用预置的出站 Address
	if _, err := s.handle.Send(buf.Bytes(), &s.addr); err != nil {
		return time.Time{}, err
	}
	return start, nil
}
