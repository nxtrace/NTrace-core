//go:build !darwin && !(windows && amd64)

package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"github.com/nxtrace/NTrace-core/util"
)

type UDPSpec struct {
	IPVersion    int
	ICMPMode     int
	SrcIP        net.IP
	DstIP        net.IP
	DstPort      int
	icmp         net.PacketConn
	udp          net.PacketConn
	udp4         *ipv4.RawConn
	udp6         *ipv6.PacketConn
	hopLimitLock sync.Mutex
	mtu          int
}

func (s *UDPSpec) InitUDP() {
	network := "ip4:udp"
	if s.IPVersion == 6 {
		network = "ip6:udp"
	}

	udp, err := net.ListenPacket(network, s.SrcIP.String())
	if err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(InitUDP) ListenPacket(%s, %s) failed: %v", network, s.SrcIP, err))
		}
		log.Fatalf("(InitUDP) ListenPacket(%s, %s) failed: %v", network, s.SrcIP, err)
	}
	s.udp = udp

	if s.IPVersion == 4 {
		s.udp4, err = ipv4.NewRawConn(s.udp)
		if err != nil {
			s.Close()
			if util.EnvDevMode {
				panic(fmt.Errorf("(InitUDP) create NewRawConn failed: %v", err))
			}
			log.Fatalf("(InitUDP) create NewRawConn failed: %v", err)
		}

		// 获取本地接口的 MTU
		mtu := 1500
		if m := util.GetMTUByIP(s.SrcIP); m > 0 {
			mtu = m
		}
		s.mtu = mtu
	} else {
		s.udp6 = ipv6.NewPacketConn(s.udp)
	}
}

func (s *UDPSpec) Close() {
	_ = s.icmp.Close()
	_ = s.udp.Close()
}

func (s *UDPSpec) ListenOut(_ context.Context, _ chan struct{}, _ func(srcPort, seq, ttl int, start time.Time)) {
}

func (s *UDPSpec) ListenICMP(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, data []byte)) {
	s.listenICMPSock(ctx, ready, onICMP)
}

func (s *UDPSpec) SendUDP(ctx context.Context, ipHdr ipLayer, udpHdr *layers.UDP, payload []byte) (time.Time, error) {
	select {
	case <-ctx.Done():
		return time.Time{}, context.Canceled
	default:
	}

	if s.IPVersion == 4 {
		ip4, ok := ipHdr.(*layers.IPv4)
		if !ok || ip4 == nil {
			return time.Time{}, errors.New("SendUDP: expect *layers.IPv4 when s.IPVersion==4")
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

		// 完整的报文字节
		packet := buf.Bytes()
		total := len(packet)

		// 解析 IP 头长度
		ihl := int(packet[0]&0x0f) * 4

		// 从序列化后的整包中切分出 IP 头和负载（UDP 头 + payload）
		hdr, err := ipv4.ParseHeader(packet[:ihl])
		if err != nil {
			return time.Time{}, err
		}
		body := packet[ihl:]

		var start time.Time // 记录起始时间
		if total <= s.mtu {
			// (1) 不分片：总长 ≤ MTU，直接发送
			start = time.Now()
			if err := s.udp4.WriteTo(hdr, body, nil); err != nil {
				return time.Time{}, err
			}
		} else {
			// (2) 分片：总长 > MTU，调用 util.IPv4Fragmentize
			frags, err := util.IPv4Fragmentize(hdr, body, s.mtu)
			if err != nil {
				return time.Time{}, err
			}

			start = time.Now()
			for _, fr := range frags {
				if err := s.udp4.WriteTo(&fr.Hdr, fr.Body, nil); err != nil {
					return time.Time{}, err
				}
			}
		}
		return start, nil
	}

	ip6, ok := ipHdr.(*layers.IPv6)
	if !ok || ip6 == nil {
		return time.Time{}, errors.New("SendUDP: expect *layers.IPv6 when s.IPVersion==6")
	}
	ttl := int(ip6.HopLimit)

	_ = udpHdr.SetNetworkLayerForChecksum(ipHdr)

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	// 序列化 UDP 头与 payload 到缓冲区
	if err := gopacket.SerializeLayers(buf, opts, udpHdr, gopacket.Payload(payload)); err != nil {
		return time.Time{}, err
	}

	// 串行设置 HopLimit + 发送，放在同一把锁里保证并发安全
	s.hopLimitLock.Lock()
	defer s.hopLimitLock.Unlock()

	if err := s.udp6.SetHopLimit(ttl); err != nil {
		return time.Time{}, err
	}

	start := time.Now()

	if _, err := s.udp.WriteTo(buf.Bytes(), &net.IPAddr{IP: s.DstIP}); err != nil {
		return time.Time{}, err
	}
	return start, nil
}
