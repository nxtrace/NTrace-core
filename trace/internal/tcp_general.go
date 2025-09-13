//go:build !darwin && !windows

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
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"github.com/nxtrace/NTrace-core/util"
)

type TCPSpec struct {
	IPVersion    int
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

func NewTCPSpec(ipv int, srcIP, dstIP net.IP, dstPort int, icmp net.PacketConn, pktSize int) *TCPSpec {
	return &TCPSpec{IPVersion: ipv, SrcIP: srcIP, DstIP: dstIP, DstPort: dstPort, icmp: icmp, PktSize: pktSize}
}

func (s *TCPSpec) InitTCP() {
	network := "ip4:tcp"
	if s.IPVersion == 6 {
		network = "ip6:tcp"
	}

	tcp, err := net.ListenPacket(network, s.SrcIP.String())
	if err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("InitTCP: ListenPacket(%s, %s) failed: %v", network, s.SrcIP, err))
		}
		log.Fatalf("InitTCP: ListenPacket(%s, %s) failed: %v", network, s.SrcIP, err)
	}
	s.tcp = tcp

	if s.IPVersion == 4 {
		s.tcp4 = ipv4.NewPacketConn(tcp)
	} else {
		s.tcp6 = ipv6.NewPacketConn(tcp)
	}
}

func (s *TCPSpec) Close() {
	_ = s.tcp.Close()
}

func (s *TCPSpec) ListenICMP(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, data []byte)) {
	lc := NewPacketListener(s.icmp)
	go lc.Start(ctx)
	close(ready)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-lc.Messages:
			if !ok {
				return
			}

			if msg.Err != nil {
				continue
			}
			finish := time.Now()

			var data []byte // 提取 ICMP 负载
			if s.IPVersion == 4 {
				rm, err := icmp.ParseMessage(1, msg.Msg)
				if err != nil {
					continue
				}

				switch rm.Type {
				case ipv4.ICMPTypeTimeExceeded:
					if body, ok := rm.Body.(*icmp.TimeExceeded); ok && body != nil {
						data = body.Data
					}
				case ipv4.ICMPTypeDestinationUnreachable:
					if body, ok := rm.Body.(*icmp.DstUnreach); ok && body != nil {
						data = body.Data
					}
				default:
					//log.Println("received icmp message of unknown type", rm.Type)
					continue
				}

				if len(data) < 20 || data[0]>>4 != 4 {
					continue
				}

				dstIP := net.IP(data[16:20])
				if !(dstIP.Equal(s.DstIP) || dstIP.Equal(net.IPv4zero)) {
					continue
				}
			} else {
				rm, err := icmp.ParseMessage(58, msg.Msg)
				if err != nil {
					continue
				}

				switch rm.Type {
				case ipv6.ICMPTypeTimeExceeded:
					if body, ok := rm.Body.(*icmp.TimeExceeded); ok && body != nil {
						data = body.Data
					}
				case ipv6.ICMPTypePacketTooBig:
					if body, ok := rm.Body.(*icmp.PacketTooBig); ok && body != nil {
						data = body.Data
					}
				case ipv6.ICMPTypeDestinationUnreachable:
					if body, ok := rm.Body.(*icmp.DstUnreach); ok && body != nil {
						data = body.Data
					}
				default:
					//log.Println("received icmp message of unknown type", rm.Type)
					continue
				}

				if len(data) < 40 || data[0]>>4 != 6 {
					continue
				}

				dstIP := net.IP(data[24:40])
				if !(dstIP.Equal(s.DstIP) || dstIP.Equal(net.IPv6zero)) {
					continue
				}
			}
			onICMP(msg, finish, data)
		}
	}
}

func (s *TCPSpec) ListenTCP(ctx context.Context, ready chan struct{}, onTCP func(srcPort, seq int, peer net.Addr, finish time.Time)) {
	lc := NewPacketListener(s.tcp)
	go lc.Start(ctx)
	close(ready)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-lc.Messages:
			if !ok {
				return
			}

			if msg.Err != nil {
				continue
			}
			finish := time.Now()

			if ip := util.AddrIP(msg.Peer); ip == nil || !ip.Equal(s.DstIP) {
				continue
			}

			// 解包
			packet := gopacket.NewPacket(msg.Msg, layers.LayerTypeTCP, gopacket.Default)
			if packet.ErrorLayer() != nil {
				continue
			}

			// 从包中获取 TCP 层信息
			tl, ok := packet.Layer(layers.LayerTypeTCP).(*layers.TCP)
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
			srcPort := int(tl.DstPort)
			onTCP(srcPort, seq, msg.Peer, finish)
		}
	}
}

func (s *TCPSpec) SendTCP(ctx context.Context, ipHdr interface{}, tcpHdr *layers.TCP, payload []byte) (time.Time, error) {
	select {
	case <-ctx.Done():
		return time.Time{}, context.Canceled
	default:
	}

	// 统一持有网络层接口
	var (
		netL gopacket.NetworkLayer
		ttl  int
	)
	if s.IPVersion == 4 {
		ip4, ok := ipHdr.(*layers.IPv4)
		if !ok || ip4 == nil {
			return time.Time{}, errors.New("SendTCP: expect *layers.IPv4 when ipv==4")
		}
		netL, ttl = ip4, int(ip4.TTL)
	} else {
		ip6, ok := ipHdr.(*layers.IPv6)
		if !ok || ip6 == nil {
			return time.Time{}, errors.New("SendTCP: expect *layers.IPv6 when ipv==6")
		}
		netL, ttl = ip6, int(ip6.HopLimit)
	}

	_ = tcpHdr.SetNetworkLayerForChecksum(netL)

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	// 序列化 TCP 头与 payload 到缓冲区
	if err := gopacket.SerializeLayers(buf, opts, tcpHdr, gopacket.Payload(payload)); err != nil {
		return time.Time{}, err
	}

	// 串行设置 TTL/HopLimit + 发送，放在同一把锁里保证并发安全
	s.hopLimitLock.Lock()
	if s.IPVersion == 4 {
		if err := s.tcp4.SetTTL(ttl); err != nil {
			s.hopLimitLock.Unlock()
			return time.Time{}, err
		}
	} else {
		if err := s.tcp6.SetHopLimit(ttl); err != nil {
			s.hopLimitLock.Unlock()
			return time.Time{}, err
		}
	}

	start := time.Now()

	if _, err := s.tcp.WriteTo(buf.Bytes(), &net.IPAddr{IP: s.DstIP}); err != nil {
		s.hopLimitLock.Unlock()
		return time.Time{}, err
	}
	s.hopLimitLock.Unlock()
	return start, nil
}
