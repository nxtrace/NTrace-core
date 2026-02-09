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

type TCPSpec struct {
	IPVersion int
	ICMPMode  int
	SrcIP     net.IP
	DstIP     net.IP
	DstPort   int
	icmp      net.PacketConn
	PktSize   int
	addr      wd.Address
	handle    wd.Handle
}

func (s *TCPSpec) InitTCP() {
	handle, err := wd.Open("false", wd.LayerNetwork, 0, 0)
	if err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(InitTCP) WinDivert open failed: %v", err))
		}
		log.Fatalf("(InitTCP) WinDivert open failed: %v", err)
	}
	s.handle = handle

	// 设置出站 Address
	s.addr.SetLayer(wd.LayerNetwork)
	s.addr.SetEvent(wd.EventNetworkPacket)
	s.addr.SetOutbound()
}

func (s *TCPSpec) Close() {
	_ = s.icmp.Close()
	_ = s.handle.Close()
}

// resolveICMPMode 进行最终模式判定
func (s *TCPSpec) resolveICMPMode() int {
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

func (s *TCPSpec) ListenICMP(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, data []byte)) {
	switch s.resolveICMPMode() {
	case 1:
		s.listenICMPSock(ctx, ready, onICMP)
	case 2:
		s.listenICMPWinDivert(ctx, ready, onICMP)
	}
}

func (s *TCPSpec) listenICMPWinDivert(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, data []byte)) {
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

func (s *TCPSpec) ListenTCP(ctx context.Context, ready chan struct{}, onTCP func(srcPort, seq int, peer net.Addr, finish time.Time)) {
	// 构造 WinDivert 过滤器：入站 TCP，来自目标 s.DstIP → 本机 s.SrcIP，且源端口为 s.DstPort
	var filter string
	if s.IPVersion == 4 {
		filter = fmt.Sprintf(
			"inbound and tcp and ip.SrcAddr == %s and ip.DstAddr == %s and tcp.SrcPort == %d",
			s.DstIP.String(), s.SrcIP.String(), s.DstPort,
		)
	} else {
		filter = fmt.Sprintf(
			"inbound and tcp and ipv6.SrcAddr == %s and ipv6.DstAddr == %s and tcp.SrcPort == %d",
			s.DstIP.String(), s.SrcIP.String(), s.DstPort,
		)
	}

	// 以嗅探模式打开 WinDivert：只复制匹配的包，不拦截
	sniffHandle, err := wd.Open(filter, wd.LayerNetwork, 0, wd.FlagSniff|wd.FlagRecvOnly)
	if err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(ListenTCP) WinDivert open failed: %v (filter=%q)", err, filter))
		}
		log.Fatalf("(ListenTCP) WinDivert open failed: %v (filter=%q)", err, filter)
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

		// 从包中获取 TCP 层信息
		tl, ok := pkt.Layer(layers.LayerTypeTCP).(*layers.TCP)
		if !ok || tl == nil {
			continue
		}

		if int(tl.SrcPort) != s.DstPort {
			continue
		}

		// 依据报文类型还原原始探测 seq：1=RST+ACK => ack-1-s.PktSize；2=SYN+ACK => ack-1
		var seq int
		if tl.ACK && tl.RST {
			seq = int(tl.Ack) - 1 - s.PktSize
		} else if tl.ACK && tl.SYN {
			seq = int(tl.Ack) - 1
		} else {
			continue
		}

		var peerIP net.IP
		if s.IPVersion == 4 {
			ip4, ok := pkt.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
			if !ok || ip4 == nil {
				continue
			}
			peerIP = ip4.SrcIP
		} else {
			ip6, ok := pkt.Layer(layers.LayerTypeIPv6).(*layers.IPv6)
			if !ok || ip6 == nil {
				continue
			}
			peerIP = ip6.SrcIP
		}
		peer := &net.IPAddr{IP: peerIP}
		srcPort := int(tl.DstPort)
		onTCP(srcPort, seq, peer, finish)
	}
}

func (s *TCPSpec) SendTCP(ctx context.Context, ipHdr ipLayer, tcpHdr *layers.TCP, payload []byte) (time.Time, error) {
	select {
	case <-ctx.Done():
		return time.Time{}, context.Canceled
	default:
	}

	_ = tcpHdr.SetNetworkLayerForChecksum(ipHdr)

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	// 序列化 IP 与 TCP 头以及 payload 到缓冲区
	if err := gopacket.SerializeLayers(buf, opts, ipHdr, tcpHdr, gopacket.Payload(payload)); err != nil {
		return time.Time{}, err
	}

	start := time.Now()

	// 复用预置的出站 Address
	if _, err := s.handle.Send(buf.Bytes(), &s.addr); err != nil {
		return time.Time{}, err
	}
	return start, nil
}
