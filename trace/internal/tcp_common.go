package internal

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"github.com/nxtrace/NTrace-core/util"
)

func NewTCPSpec(IPVersion, ICMPMode int, srcIP, dstIP net.IP, dstPort int, pktSize int) *TCPSpec {
	return &TCPSpec{IPVersion: IPVersion, ICMPMode: ICMPMode, SrcIP: srcIP, DstIP: dstIP, DstPort: dstPort, PktSize: pktSize}
}

func (s *TCPSpec) InitICMP() {
	network := "ip4:icmp"
	if s.IPVersion == 6 {
		network = "ip6:ipv6-icmp"
	}

	icmpConn, err := net.ListenPacket(network, s.SrcIP.String())
	if err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(InitICMP) ListenPacket(%s, %s) failed: %v", network, s.SrcIP, err))
		}
		log.Fatalf("(InitICMP) ListenPacket(%s, %s) failed: %v", network, s.SrcIP, err)
	}
	s.icmp = icmpConn
}

func (s *TCPSpec) listenICMPSock(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, data []byte)) {
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

			var data []byte // 提取 ICMP 的负载
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
				if !dstIP.Equal(s.DstIP) {
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
				if !dstIP.Equal(s.DstIP) {
					continue
				}
			}
			onICMP(msg, finish, data)
		}
	}
}
