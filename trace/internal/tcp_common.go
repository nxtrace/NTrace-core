package internal

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

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
			finish, data, ok := s.decodeICMPSocketMessage(msg)
			if ok {
				onICMP(msg, finish, data)
			}
		}
	}
}

func (s *TCPSpec) decodeICMPSocketMessage(msg ReceivedMessage) (time.Time, []byte, bool) {
	if msg.Err != nil {
		return time.Time{}, nil, false
	}

	finish := time.Now()
	rm, ok := parseSocketICMPMessage(s.IPVersion, msg.Msg)
	if !ok {
		return finish, nil, false
	}

	data, ok := extractSocketICMPPayload(s.IPVersion, rm, s.DstIP)
	return finish, data, ok
}
