//go:build darwin

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
	udp4         *ipv4.PacketConn
	udp6         *ipv6.PacketConn
	hopLimitLock sync.Mutex
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
		s.udp4 = ipv4.NewPacketConn(s.udp)
	} else {
		s.udp6 = ipv6.NewPacketConn(s.udp)
	}
}

func (s *UDPSpec) Close() {
	_ = s.icmp.Close()
	_ = s.udp.Close()
}

func (s *UDPSpec) ListenOut(ctx context.Context, ready chan struct{}, onOut func(srcPort, seq, ttl int, start time.Time)) {
	// 选择捕获设备与本地接口
	dev := "en0"
	if util.SrcDev != "" {
		dev = util.SrcDev
	} else if d, err := util.PcapDeviceByIP(s.SrcIP); err == nil {
		dev = d
	}

	// 以“立即模式”打开 pcap，降低首包丢失概率
	handle, err := util.OpenLiveImmediate(dev, 65535, true, 4<<20)
	if err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(ListenOut) pcap open failed on %s: %v", dev, err))
		}
		log.Fatalf("(ListenOut) pcap open failed on %s: %v", dev, err)
	}
	defer handle.Close()

	// 过滤：只抓 ip + udp，来自本机 s.SrcIP → 目标 s.DstIP，且目标端口为 s.DstPort
	filter := fmt.Sprintf(
		"ip and udp and src host %s and dst host %s and dst port %d",
		s.SrcIP.String(), s.DstIP.String(), s.DstPort,
	)

	if err := handle.SetBPFFilter(filter); err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(ListenOut) set BPF failed: %v (filter=%q)", err, filter))
		}
		log.Fatalf("(ListenOut) set BPF failed: %v (filter=%q)", err, filter)
	}

	src := gopacket.NewPacketSource(handle, handle.LinkType())
	pktCh := src.Packets()
	close(ready)

	for {
		select {
		case <-ctx.Done():
			return
		case pkt, ok := <-pktCh:
			if !ok {
				return
			}

			// 解包
			packet := pkt.NetworkLayer()
			if packet == nil {
				continue
			}
			start := pkt.Metadata().Timestamp

			// 从包中获取 IPv4 层信息
			ip4, ok := pkt.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
			if !ok || ip4 == nil {
				continue
			}

			// 从包中获取 UDP 层信息
			ul, ok := pkt.Layer(layers.LayerTypeUDP).(*layers.UDP)
			if !ok || ul == nil {
				continue
			}

			ttl := int(ip4.TTL)
			srcPort := int(ul.SrcPort)
			seq := int(ip4.Id)
			onOut(srcPort, seq, ttl, start)
		}
	}
}

func (s *UDPSpec) ListenICMP(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, data []byte)) {
	s.listenICMPSock(ctx, ready, onICMP)
}

func (s *UDPSpec) SendUDP(ctx context.Context, ipHdr gopacket.NetworkLayer, udpHdr *layers.UDP, payload []byte) (time.Time, error) {
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
		ttl := int(ip4.TTL)

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

		// 串行设置 TTL + 发送，放在同一把锁里保证并发安全
		s.hopLimitLock.Lock()
		defer s.hopLimitLock.Unlock()

		if err := s.udp4.SetTTL(ttl); err != nil {
			return time.Time{}, err
		}

		start := time.Now()

		if _, err := s.udp.WriteTo(buf.Bytes(), &net.IPAddr{IP: s.DstIP}); err != nil {
			return time.Time{}, err
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
