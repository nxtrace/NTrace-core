//go:build windows && amd64

package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
	"unsafe"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	wd "github.com/xjasonlyu/windivert-go"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"golang.org/x/sys/windows"

	"github.com/nxtrace/NTrace-core/util"
)

type ICMPSpec struct {
	IPVersion    int
	ICMPMode     int
	EchoID       int
	SrcIP        net.IP
	DstIP        net.IP
	icmp         net.PacketConn
	icmp4        *ipv4.PacketConn
	icmp6        *ipv6.PacketConn
	hopLimitLock sync.Mutex
}

func ListenPacket(network string, laddr string) (net.PacketConn, error) {
	return net.ListenPacket(network, laddr)
}

func (s *ICMPSpec) Close() {
	_ = s.icmp.Close()
}

// isAdmin 判断当前进程是否具有管理员权限
func isAdmin() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer func() {
		_ = token.Close()
	}()

	type tokenElevation struct {
		TokenIsElevated uint32
	}
	var elev tokenElevation
	var outLen uint32

	if err := windows.GetTokenInformation(
		token,
		windows.TokenElevation,
		(*byte)(unsafe.Pointer(&elev)),
		uint32(unsafe.Sizeof(elev)),
		&outLen,
	); err != nil {
		return false
	}
	return elev.TokenIsElevated != 0
}

// winDivertAvailable 通过尝试打开一个 WinDivert 嗅探 handle 来判断 WinDivert 是否可用
func winDivertAvailable() (bool, error) {
	h, err := wd.Open("false", wd.LayerNetwork, 0, wd.FlagSniff|wd.FlagRecvOnly)
	if err != nil {
		return false, fmt.Errorf("WinDivert not available: %v", err)
	}
	_ = h.Close()
	return true, nil
}

// resolveICMPMode 进行最终模式判定
// 1=Socket, 2=WinDivert (嗅探模式，原 PCAP 模式的替代)
func (s *ICMPSpec) resolveICMPMode() int {
	icmpMode := s.ICMPMode
	if icmpMode != 1 && icmpMode != 2 {
		icmpMode = 0 // 统一成 Auto
	}

	// 指定 1=Socket：直接返回
	if icmpMode == 1 {
		return 1
	}

	// Auto(0) 或强制 Sniff(2) → 尝试 WinDivert
	if !isAdmin() {
		if icmpMode == 2 {
			log.Printf("WinDivert sniff mode requested, but administrator privilege is required; falling back to Socket mode.")
		}
		return 1
	}

	ok, err := winDivertAvailable()
	if !ok {
		if icmpMode == 2 {
			log.Printf("WinDivert sniff mode requested, but WinDivert is not available: %v; falling back to Socket mode.", err)
		}
		return 1
	}
	return 2
}

func (s *ICMPSpec) ListenICMP(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, seq int)) {
	switch s.resolveICMPMode() {
	case 1:
		s.listenICMPSock(ctx, ready, onICMP)
	case 2:
		s.listenICMPWinDivert(ctx, ready, onICMP)
	}
}

func (s *ICMPSpec) listenICMPWinDivert(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, seq int)) {
	// 构造 WinDivert 过滤器：入站 ICMP/ICMPv6，目标为本机 s.SrcIP
	var filter string
	if s.IPVersion == 4 {
		filter = fmt.Sprintf("inbound and icmp and ip.DstAddr == %s", s.SrcIP.String())
	} else {
		filter = fmt.Sprintf("inbound and icmpv6 and ipv6.DstAddr == %s", s.SrcIP.String())
	}

	// 以嗅探模式（FlagSniff）打开 WinDivert：只复制匹配的包，不拦截
	handle, err := wd.Open(filter, wd.LayerNetwork, 0, wd.FlagSniff|wd.FlagRecvOnly)
	if err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(ListenICMP) WinDivert open failed: %v (filter=%q)", err, filter))
		}
		log.Fatalf("(ListenICMP) WinDivert open failed: %v (filter=%q)", err, filter)
	}
	defer handle.Close()

	// 增大队列防丢包
	_ = handle.SetParam(wd.QueueLength, 8192)
	_ = handle.SetParam(wd.QueueTime, 4000) // 4s

	close(ready)

	buf := make([]byte, 65535)
	var addr wd.Address

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := handle.Recv(buf, &addr)
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

		// WinDivert 返回的数据从 IP 头开始，直接用 gopacket 解码
		var firstLayer gopacket.Decoder
		if s.IPVersion == 4 {
			firstLayer = layers.LayerTypeIPv4
		} else {
			firstLayer = layers.LayerTypeIPv6
		}
		pkt := gopacket.NewPacket(raw, firstLayer, gopacket.NoCopy)

		// outer = 完整 IP 报文字节（WinDivert 已从 IP 头开始）
		outer := raw

		var peerIP net.IP // 提取对端 IP（按族别）
		var data []byte   // 提取 ICMP 的负载
		if s.IPVersion == 4 {
			// 从包中获取 IPv4 层信息
			ip4, ok := pkt.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
			if !ok || ip4 == nil {
				continue
			}
			peerIP = ip4.SrcIP

			// 从包中获取 ICMPv4 层信息
			ic4, ok := pkt.Layer(layers.LayerTypeICMPv4).(*layers.ICMPv4)
			if !ok || ic4 == nil {
				continue
			}
			data = ic4.Payload

			switch ic4.TypeCode.Type() {
			case layers.ICMPv4TypeEchoReply:
				if !peerIP.Equal(s.DstIP) {
					continue
				}

				id := int(ic4.Id)
				if id != s.EchoID {
					continue
				}
				peer := &net.IPAddr{IP: peerIP}

				msg := ReceivedMessage{
					Peer: peer,
					Msg:  outer,
				}
				seq := int(ic4.Seq)
				onICMP(msg, finish, seq)
				continue
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
			// 从包中获取 IPv6 层信息
			ip6, ok := pkt.Layer(layers.LayerTypeIPv6).(*layers.IPv6)
			if !ok || ip6 == nil {
				continue
			}
			peerIP = ip6.SrcIP

			// 从包中获取 ICMPv6 层信息
			ic6, ok := pkt.Layer(layers.LayerTypeICMPv6).(*layers.ICMPv6)
			if !ok || ic6 == nil {
				continue
			}
			data = ic6.Payload[4:]

			switch ic6.TypeCode.Type() {
			case layers.ICMPv6TypeEchoReply:
				echo, ok := pkt.Layer(layers.LayerTypeICMPv6Echo).(*layers.ICMPv6Echo)
				if !ok || echo == nil {
					continue
				}

				if !peerIP.Equal(s.DstIP) {
					continue
				}

				id := int(echo.Identifier)
				if id != s.EchoID {
					continue
				}
				peer := &net.IPAddr{IP: peerIP}

				msg := ReceivedMessage{
					Peer: peer,
					Msg:  outer,
				}
				seq := int(echo.SeqNumber)
				onICMP(msg, finish, seq)
				continue
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

		header, err := util.GetICMPResponsePayload(data)
		if err != nil {
			continue
		}

		id, err := util.GetICMPID(header)
		if err != nil || id != s.EchoID {
			continue
		}

		seq, err := util.GetICMPSeq(header)
		if err != nil {
			continue
		}
		onICMP(msg, finish, seq)
	}
}

func (s *ICMPSpec) SendICMP(ctx context.Context, ipHdr gopacket.NetworkLayer, icmpHdr, icmpEcho gopacket.SerializableLayer, payload []byte) (time.Time, error) {
	select {
	case <-ctx.Done():
		return time.Time{}, context.Canceled
	default:
	}

	if s.IPVersion == 4 {
		ip4, ok := ipHdr.(*layers.IPv4)
		if !ok || ip4 == nil {
			return time.Time{}, errors.New("SendICMP: expect *layers.IPv4 when s.IPVersion==4")
		}
		ttl := int(ip4.TTL)

		buf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{
			ComputeChecksums: true,
			FixLengths:       true,
		}

		// 序列化 ICMP 头与 payload 到缓冲区
		if err := gopacket.SerializeLayers(buf, opts, icmpHdr, gopacket.Payload(payload)); err != nil {
			return time.Time{}, err
		}

		// 串行设置 TTL + 发送，放在同一把锁里保证并发安全
		s.hopLimitLock.Lock()
		defer s.hopLimitLock.Unlock()

		if err := s.icmp4.SetTTL(ttl); err != nil {
			return time.Time{}, err
		}

		start := time.Now()

		if _, err := s.icmp.WriteTo(buf.Bytes(), &net.IPAddr{IP: s.DstIP}); err != nil {
			return time.Time{}, err
		}
		return start, nil
	}

	ip6, ok := ipHdr.(*layers.IPv6)
	if !ok || ip6 == nil {
		return time.Time{}, errors.New("SendICMP: expect *layers.IPv6 when s.IPVersion==6")
	}
	ttl := int(ip6.HopLimit)

	ic6, ok := icmpHdr.(*layers.ICMPv6)
	if !ok || ic6 == nil {
		return time.Time{}, errors.New("SendICMP: expect *layers.ICMPv6 when s.IPVersion==6")
	}

	_ = ic6.SetNetworkLayerForChecksum(ipHdr)

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	// 序列化 ICMP 头与 payload 到缓冲区
	if err := gopacket.SerializeLayers(buf, opts, icmpHdr, icmpEcho, gopacket.Payload(payload)); err != nil {
		return time.Time{}, err
	}

	// 串行设置 HopLimit + 发送，放在同一把锁里保证并发安全
	s.hopLimitLock.Lock()
	defer s.hopLimitLock.Unlock()

	if err := s.icmp6.SetHopLimit(ttl); err != nil {
		return time.Time{}, err
	}

	start := time.Now()

	if _, err := s.icmp.WriteTo(buf.Bytes(), &net.IPAddr{IP: s.DstIP}); err != nil {
		return time.Time{}, err
	}
	return start, nil
}
