//go:build !darwin && !(windows && amd64)

package internal

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
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

func (s *ICMPSpec) ListenICMP(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, seq int)) {
	s.listenICMPSock(ctx, ready, onICMP)
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
