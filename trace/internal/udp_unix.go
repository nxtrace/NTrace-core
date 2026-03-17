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
	SourceDevice string
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
		if m := util.GetMTUByIPForDevice(s.SrcIP, s.SourceDevice); m > 0 {
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

func serializeUDPPacket(payload []byte, layersToSerialize ...gopacket.SerializableLayer) ([]byte, error) {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}
	serializeLayers := append(layersToSerialize, gopacket.Payload(payload))
	if err := gopacket.SerializeLayers(buf, opts, serializeLayers...); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func parseIPv4Packet(packet []byte) (*ipv4.Header, []byte, error) {
	ihl := int(packet[0]&0x0f) * 4
	hdr, err := ipv4.ParseHeader(packet[:ihl])
	if err != nil {
		return nil, nil, err
	}
	return hdr, packet[ihl:], nil
}

func (s *UDPSpec) sendUDPIPv4(ipHdr *layers.IPv4, udpHdr *layers.UDP, payload []byte) (time.Time, error) {
	if ipHdr == nil {
		return time.Time{}, errors.New("SendUDP: expect *layers.IPv4 when s.IPVersion==4")
	}
	if err := udpHdr.SetNetworkLayerForChecksum(ipHdr); err != nil {
		return time.Time{}, fmt.Errorf("SetNetworkLayerForChecksum: %w", err)
	}

	packet, err := serializeUDPPacket(payload, ipHdr, udpHdr)
	if err != nil {
		return time.Time{}, err
	}
	hdr, body, err := parseIPv4Packet(packet)
	if err != nil {
		return time.Time{}, err
	}

	if len(packet) <= s.mtu {
		start := time.Now()
		if err := s.udp4.WriteTo(hdr, body, nil); err != nil {
			return time.Time{}, err
		}
		return start, nil
	}

	frags, err := util.IPv4Fragmentize(hdr, body, s.mtu)
	if err != nil {
		return time.Time{}, err
	}
	start := time.Now()
	for _, fr := range frags {
		if err := s.udp4.WriteTo(&fr.Hdr, fr.Body, nil); err != nil {
			return time.Time{}, err
		}
	}
	return start, nil
}

func (s *UDPSpec) sendUDPIPv6(ipHdr *layers.IPv6, udpHdr *layers.UDP, payload []byte) (time.Time, error) {
	if ipHdr == nil {
		return time.Time{}, errors.New("SendUDP: expect *layers.IPv6 when s.IPVersion==6")
	}
	if err := udpHdr.SetNetworkLayerForChecksum(ipHdr); err != nil {
		return time.Time{}, fmt.Errorf("SetNetworkLayerForChecksum: %w", err)
	}

	packet, err := serializeUDPPacket(payload, udpHdr)
	if err != nil {
		return time.Time{}, err
	}

	s.hopLimitLock.Lock()
	defer s.hopLimitLock.Unlock()

	if err := s.udp6.SetTrafficClass(int(ipHdr.TrafficClass)); err != nil {
		return time.Time{}, err
	}
	if err := s.udp6.SetHopLimit(int(ipHdr.HopLimit)); err != nil {
		return time.Time{}, err
	}
	start := time.Now()
	if _, err := s.udp.WriteTo(packet, &net.IPAddr{IP: s.DstIP}); err != nil {
		return time.Time{}, err
	}
	return start, nil
}

func (s *UDPSpec) SendUDP(ctx context.Context, ipHdr ipLayer, udpHdr *layers.UDP, payload []byte) (time.Time, error) {
	select {
	case <-ctx.Done():
		return time.Time{}, context.Canceled
	default:
	}

	if s.IPVersion == 4 {
		ip4, _ := ipHdr.(*layers.IPv4)
		return s.sendUDPIPv4(ip4, udpHdr, payload)
	}

	ip6, _ := ipHdr.(*layers.IPv6)
	return s.sendUDPIPv6(ip6, udpHdr, payload)
}
