//go:build !darwin && !(windows && amd64)

package internal

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type TCPSpec struct {
	IPVersion    int
	ICMPMode     int
	SrcIP        net.IP
	DstIP        net.IP
	DstPort      int
	PktSize      int
	SourceDevice string
	FWMark       uint32
	FWMarkSet    bool
	icmp         net.PacketConn
	tcp          net.PacketConn
	tcp4         *ipv4.PacketConn
	tcp6         *ipv6.PacketConn
	hopLimitLock sync.Mutex
}

func (s *TCPSpec) InitTCP() error {
	network := "ip4:tcp"
	if s.IPVersion == 6 {
		network = "ip6:tcp"
	}

	tcp, err := net.ListenPacket(network, s.SrcIP.String())
	if err != nil {
		return fmt.Errorf("(InitTCP) ListenPacket(%s, %s) failed: %w", network, s.SrcIP, err)
	}
	if err := setPacketConnFWMark(tcp, s.FWMark, s.FWMarkSet); err != nil {
		_ = tcp.Close()
		return err
	}
	if s.SourceDevice != "" {
		if err := bindPacketConnToSourceDevice(tcp, s.IPVersion, s.SourceDevice); err != nil {
			_ = tcp.Close()
			return fmt.Errorf("(InitTCP) bind source device %q failed: %w", s.SourceDevice, err)
		}
	}
	s.tcp = tcp

	if s.IPVersion == 4 {
		s.tcp4 = ipv4.NewPacketConn(s.tcp)
	} else {
		s.tcp6 = ipv6.NewPacketConn(s.tcp)
	}
	return nil
}

func (s *TCPSpec) Close() {
	if s.icmp != nil {
		_ = s.icmp.Close()
	}
	if s.tcp != nil {
		_ = s.tcp.Close()
	}
}

func (s *TCPSpec) ListenICMP(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, data []byte)) error {
	return s.listenICMPSock(ctx, ready, onICMP)
}

func (s *TCPSpec) ListenTCP(ctx context.Context, ready chan struct{}, onTCP func(srcPort, seq, ack int, peer net.Addr, finish time.Time)) error {
	lc, stop := startPacketListener(ctx, s.tcp)
	defer stop()
	close(ready)

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-lc.Messages:
			if !ok {
				return listenerStoppedError(ctx)
			}

			if msg.Err != nil {
				return msg.Err
			}
			finish := time.Now()

			// 解包
			packet := gopacket.NewPacket(msg.Msg, layers.LayerTypeTCP, gopacket.Default)
			if packet.ErrorLayer() != nil {
				continue
			}

			// 从包中获取 TCP 层信息
			tl, ok := packet.Layer(layers.LayerTypeTCP).(*layers.TCP)
			if !ok || tl == nil || int(tl.SrcPort) != s.DstPort {
				continue
			}

			seq, ack, ok := tcpProbeReply(tl)
			if !ok {
				continue
			}

			srcPort := int(tl.DstPort)
			onTCP(srcPort, seq, ack, msg.Peer, finish)
		}
	}
}

func (s *TCPSpec) SendTCP(ctx context.Context, ipHdr gopacket.NetworkLayer, tcpHdr *layers.TCP, payload []byte) (time.Time, error) {
	select {
	case <-ctx.Done():
		return time.Time{}, context.Canceled
	default:
	}

	if s.IPVersion == 4 {
		ip4, ok := ipHdr.(*layers.IPv4)
		if !ok || ip4 == nil {
			return time.Time{}, errors.New("SendTCP: expect *layers.IPv4 when s.IPVersion==4")
		}
		ttl := int(ip4.TTL)

		if err := tcpHdr.SetNetworkLayerForChecksum(ipHdr); err != nil {
			return time.Time{}, fmt.Errorf("SetNetworkLayerForChecksum: %w", err)
		}

		buf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{
			ComputeChecksums: true,
			FixLengths:       true,
		}

		// 序列化 TCP 头与 payload 到缓冲区
		if err := gopacket.SerializeLayers(buf, opts, tcpHdr, gopacket.Payload(payload)); err != nil {
			return time.Time{}, err
		}

		// 串行设置 TTL + 发送，放在同一把锁里保证并发安全
		s.hopLimitLock.Lock()
		defer s.hopLimitLock.Unlock()

		if err := s.tcp4.SetTOS(int(ip4.TOS)); err != nil {
			return time.Time{}, &InitializationError{Err: fmt.Errorf("set IPv4 TOS %d: %w", ip4.TOS, err)}
		}
		if err := s.tcp4.SetTTL(ttl); err != nil {
			return time.Time{}, err
		}

		start := time.Now()

		if _, err := s.tcp.WriteTo(buf.Bytes(), &net.IPAddr{IP: s.DstIP}); err != nil {
			return time.Time{}, err
		}
		return start, nil
	}

	ip6, ok := ipHdr.(*layers.IPv6)
	if !ok || ip6 == nil {
		return time.Time{}, errors.New("SendTCP: expect *layers.IPv6 when s.IPVersion==6")
	}
	ttl := int(ip6.HopLimit)

	if err := tcpHdr.SetNetworkLayerForChecksum(ipHdr); err != nil {
		return time.Time{}, fmt.Errorf("SetNetworkLayerForChecksum: %w", err)
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	// 序列化 TCP 头与 payload 到缓冲区
	if err := gopacket.SerializeLayers(buf, opts, tcpHdr, gopacket.Payload(payload)); err != nil {
		return time.Time{}, err
	}

	// 串行设置 HopLimit + 发送，放在同一把锁里保证并发安全
	s.hopLimitLock.Lock()
	defer s.hopLimitLock.Unlock()

	if err := setIPv6TrafficClass(s.tcp6, ip6.TrafficClass); err != nil {
		return time.Time{}, &InitializationError{Err: fmt.Errorf("set IPv6 Traffic Class %d: %w", ip6.TrafficClass, err)}
	}
	if err := s.tcp6.SetHopLimit(ttl); err != nil {
		return time.Time{}, err
	}

	start := time.Now()

	if _, err := s.tcp.WriteTo(buf.Bytes(), &net.IPAddr{IP: s.DstIP}); err != nil {
		return time.Time{}, err
	}
	return start, nil
}
