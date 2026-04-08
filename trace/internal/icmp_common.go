package internal

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/google/gopacket"
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
	if s.SourceDevice != "" {
		if err := bindPacketConnToSourceDevice(icmpConn, s.IPVersion, s.SourceDevice); err != nil {
			_ = icmpConn.Close()
			if util.EnvDevMode {
				panic(fmt.Errorf("(InitICMP) bind source device %q failed: %v", s.SourceDevice, err))
			}
			log.Fatalf("(InitICMP) bind source device %q failed: %v", s.SourceDevice, err)
		}
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
			finish, seq, ok := s.decodeICMPSocketMessage(msg)
			if ok {
				onICMP(msg, finish, seq)
			}
		}
	}
}

func (s *ICMPSpec) decodeICMPSocketMessage(msg ReceivedMessage) (time.Time, int, bool) {
	if msg.Err != nil {
		return time.Time{}, 0, false
	}

	finish := time.Now()
	rm, ok := parseSocketICMPMessage(s.IPVersion, msg.Msg)
	if !ok {
		return finish, 0, false
	}

	if seq, ok := matchSocketICMPEchoReply(s.IPVersion, rm, util.AddrIP(msg.Peer), s.DstIP, s.EchoID); ok {
		return finish, seq, true
	}

	data, ok := extractSocketICMPPayload(s.IPVersion, rm, s.DstIP)
	if !ok {
		return finish, 0, false
	}

	seq, ok := extractEmbeddedICMPSeq(data, s.EchoID)
	return finish, seq, ok
}
