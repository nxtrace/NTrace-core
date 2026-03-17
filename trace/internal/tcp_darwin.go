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
	"github.com/google/gopacket/pcap"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"github.com/nxtrace/NTrace-core/util"
)

type TCPSpec struct {
	IPVersion    int
	ICMPMode     int
	SrcIP        net.IP
	DstIP        net.IP
	DstPort      int
	PktSize      int
	SourceDevice string
	icmp         net.PacketConn
	tcp          net.PacketConn
	tcp4         *ipv4.PacketConn
	tcp6         *ipv6.PacketConn
	hopLimitLock sync.Mutex
}

func (s *TCPSpec) InitTCP() {
	network := "ip4:tcp"
	if s.IPVersion == 6 {
		network = "ip6:tcp"
	}

	tcp, err := net.ListenPacket(network, s.SrcIP.String())
	if err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(InitTCP) ListenPacket(%s, %s) failed: %v", network, s.SrcIP, err))
		}
		log.Fatalf("(InitTCP) ListenPacket(%s, %s) failed: %v", network, s.SrcIP, err)
	}
	s.tcp = tcp

	if s.IPVersion == 4 {
		s.tcp4 = ipv4.NewPacketConn(s.tcp)
	} else {
		s.tcp6 = ipv6.NewPacketConn(s.tcp)
	}
}

func (s *TCPSpec) Close() {
	_ = s.icmp.Close()
	_ = s.tcp.Close()
}

func (s *TCPSpec) ListenICMP(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, data []byte)) {
	s.listenICMPSock(ctx, ready, onICMP)
}

func (s *TCPSpec) captureDevice() string {
	if s.SourceDevice != "" {
		return s.SourceDevice
	}
	if dev, err := util.PcapDeviceByIP(s.SrcIP); err == nil {
		return dev
	}
	return "en0"
}

func (s *TCPSpec) tcpCaptureFilter() string {
	return fmt.Sprintf(
		"%s and tcp and src host %s and dst host %s and src port %d",
		tcpIPVersionPrefix(s.IPVersion), s.DstIP.String(), s.SrcIP.String(), s.DstPort,
	)
}

func mustOpenDarwinTCPSniffHandle(dev string) *pcap.Handle {
	handle, err := util.OpenLiveImmediate(dev, 65535, true, 4<<20)
	if err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(ListenTCP) pcap open failed on %s: %v", dev, err))
		}
		log.Fatalf("(ListenTCP) pcap open failed on %s: %v", dev, err)
	}
	return handle
}

func mustSetDarwinTCPFilter(handle *pcap.Handle, filter string) {
	if err := handle.SetBPFFilter(filter); err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(ListenTCP) set BPF failed: %v (filter=%q)", err, filter))
		}
		log.Fatalf("(ListenTCP) set BPF failed: %v (filter=%q)", err, filter)
	}
}

func (s *TCPSpec) ListenTCP(ctx context.Context, ready chan struct{}, onTCP func(srcPort, seq, ack int, peer net.Addr, finish time.Time)) {
	handle := mustOpenDarwinTCPSniffHandle(s.captureDevice())
	defer handle.Close()

	mustSetDarwinTCPFilter(handle, s.tcpCaptureFilter())
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
			finish := pkt.Metadata().Timestamp
			srcPort, seq, ack, peer, ok := decodeTCPProbePacket(s.IPVersion, s.DstPort, pkt)
			if !ok {
				continue
			}
			onTCP(srcPort, seq, ack, peer, finish)
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
			return time.Time{}, err
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

	if err := s.tcp6.SetTrafficClass(int(ip6.TrafficClass)); err != nil {
		return time.Time{}, err
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
