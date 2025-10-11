package internal

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/google/gopacket"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"github.com/nxtrace/NTrace-core/util"
)

type ipLayer interface {
	gopacket.NetworkLayer
	gopacket.SerializableLayer
}

func NewICMPSpec(IPVersion, ICMPMode, echoID int, srcIP, dstIP net.IP) *ICMPSpec {
	return &ICMPSpec{IPVersion: IPVersion, ICMPMode: ICMPMode, EchoID: echoID, SrcIP: srcIP, DstIP: dstIP}
}

func (s *ICMPSpec) InitICMP() {
	network := "ip4:icmp"
	if s.IPVersion == 6 {
		network = "ip6:ipv6-icmp"
	}

	icmpConn, err := ListenPacket(network, s.SrcIP.String())
	if err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(InitICMP) ListenPacket(%s, %s) failed: %v", network, s.SrcIP, err))
		}
		log.Fatalf("(InitICMP) ListenPacket(%s, %s) failed: %v", network, s.SrcIP, err)
	}
	s.icmp = icmpConn

	if s.IPVersion == 4 {
		s.icmp4 = ipv4.NewPacketConn(s.icmp)
	} else {
		s.icmp6 = ipv6.NewPacketConn(s.icmp)
	}
}

func (s *ICMPSpec) listenICMPSock(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, seq int)) {
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
				case ipv4.ICMPTypeEchoReply:
					echo, ok := rm.Body.(*icmp.Echo)
					if !ok || echo == nil {
						continue
					}

					if ip := util.AddrIP(msg.Peer); ip == nil || !ip.Equal(s.DstIP) {
						continue
					}

					id := echo.ID
					if id != s.EchoID {
						continue
					}

					seq := echo.Seq
					onICMP(msg, finish, seq)
					continue
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
				case ipv6.ICMPTypeEchoReply:
					echo, ok := rm.Body.(*icmp.Echo)
					if !ok || echo == nil {
						continue
					}

					if ip := util.AddrIP(msg.Peer); ip == nil || !ip.Equal(s.DstIP) {
						continue
					}

					id := echo.ID
					if id != s.EchoID {
						continue
					}

					seq := echo.Seq
					onICMP(msg, finish, seq)
					continue
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
}
