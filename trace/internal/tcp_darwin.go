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

type TCPSpec struct {
	IPVersion    int
	ICMPMode     int
	SrcIP        net.IP
	DstIP        net.IP
	DstPort      int
	PktSize      int
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

func (s *TCPSpec) ListenTCP(ctx context.Context, ready chan struct{}, onTCP func(srcPort, seq int, peer net.Addr, finish time.Time)) {
	// 选择捕获设备与本地接口
	dev := "en0"
	if util.SrcDev != "" {
		dev = util.SrcDev
	} else if d, err := util.PcapDeviceByIP(s.SrcIP); err == nil {
		dev = d
	}

	ipPrefix := "ip"
	if s.IPVersion == 6 {
		ipPrefix = "ip6"
	}

	// 以“立即模式”打开 pcap，降低首包丢失概率
	handle, err := util.OpenLiveImmediate(dev, 65535, true, 4<<20)
	if err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(ListenTCP) pcap open failed on %s: %v", dev, err))
		}
		log.Fatalf("(ListenTCP) pcap open failed on %s: %v", dev, err)
	}
	defer handle.Close()

	// 过滤：只抓 {ip/ip6} + tcp，来自目标 s.DstIP → 本机 s.SrcIP，且源端口为 s.DstPort
	filter := fmt.Sprintf(
		"%s and tcp and src host %s and dst host %s and src port %d",
		ipPrefix, s.DstIP.String(), s.SrcIP.String(), s.DstPort,
	)

	if err := handle.SetBPFFilter(filter); err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(ListenTCP) set BPF failed: %v (filter=%q)", err, filter))
		}
		log.Fatalf("(ListenTCP) set BPF failed: %v (filter=%q)", err, filter)
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
			finish := pkt.Metadata().Timestamp

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

			var peerIP net.IP // 提取对端 IP（按族别）
			if s.IPVersion == 4 {
				// 从包中获取 IPv4 层信息
				ip4, ok := pkt.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
				if !ok || ip4 == nil {
					continue
				}
				peerIP = ip4.SrcIP
			} else {
				// 从包中获取 IPv6 层信息
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

		_ = tcpHdr.SetNetworkLayerForChecksum(ipHdr)

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

	_ = tcpHdr.SetNetworkLayerForChecksum(ipHdr)

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

	if err := s.tcp6.SetHopLimit(ttl); err != nil {
		return time.Time{}, err
	}

	start := time.Now()

	if _, err := s.tcp.WriteTo(buf.Bytes(), &net.IPAddr{IP: s.DstIP}); err != nil {
		return time.Time{}, err
	}
	return start, nil
}
