//go:build windows && amd64

package internal

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/nxtrace/NTrace-core/util"
	wd "github.com/xjasonlyu/windivert-go"
)

type winDivertICMPPacket struct {
	ipVersion int
	peerIP    net.IP
	outer     []byte
	errorData []byte
	echoID    int
	echoSeq   int
	echoReply bool
}

var (
	openWinDivertSniffCall = OpenWinDivertHandle
	winDivertSniffFatal    = func(msg string) { log.Fatal(msg) }
	winDivertSniffDevMode  = func() bool { return util.EnvDevMode }
)

func winDivertICMPFilter(ipVersion int, srcIP net.IP) string {
	if ipVersion == 4 {
		return fmt.Sprintf("inbound and icmp and ip.DstAddr == %s", srcIP.String())
	}
	return fmt.Sprintf("inbound and icmpv6 and ipv6.DstAddr == %s", srcIP.String())
}

func winDivertTCPFilter(ipVersion int, dstIP, srcIP net.IP, dstPort int) string {
	if ipVersion == 4 {
		return fmt.Sprintf(
			"inbound and tcp and ip.SrcAddr == %s and ip.DstAddr == %s and tcp.SrcPort == %d",
			dstIP.String(), srcIP.String(), dstPort,
		)
	}
	return fmt.Sprintf(
		"inbound and tcp and ipv6.SrcAddr == %s and ipv6.DstAddr == %s and tcp.SrcPort == %d",
		dstIP.String(), srcIP.String(), dstPort,
	)
}

func openWinDivertSniffHandle(ctx context.Context, filter, action string) (wd.Handle, func()) {
	handle, err := openWinDivertSniffCall(filter, wd.FlagSniff|wd.FlagRecvOnly)
	if err != nil {
		msg := formatWinDivertRequiredError(fmt.Sprintf("Windows WinDivert 嗅探 (%s)", action), err)
		if winDivertSniffDevMode() {
			panic(msg)
		}
		winDivertSniffFatal(msg)
		return 0, func() {}
	}

	var closeOnce sync.Once
	closeHandle := func() { closeOnce.Do(func() { _ = handle.Close() }) }
	go func() {
		<-ctx.Done()
		closeHandle()
	}()

	_ = handle.SetParam(wd.QueueLength, 8192)
	_ = handle.SetParam(wd.QueueTime, 4000)
	return handle, closeHandle
}

func packetDecoderForIPVersion(ipVersion int) gopacket.Decoder {
	if ipVersion == 4 {
		return layers.LayerTypeIPv4
	}
	return layers.LayerTypeIPv6
}

func receiveWinDivertPacket(ctx context.Context, handle wd.Handle, buf []byte, addr *wd.Address) ([]byte, time.Time, bool) {
	select {
	case <-ctx.Done():
		return nil, time.Time{}, false
	default:
	}

	n, err := handle.Recv(buf, addr)
	if err != nil {
		select {
		case <-ctx.Done():
			return nil, time.Time{}, false
		default:
			return nil, time.Time{}, false
		}
	}

	finish := time.Now()
	raw := make([]byte, n)
	copy(raw, buf[:n])
	return raw, finish, true
}

func decodeWinDivertICMPPacket(ipVersion int, raw []byte) (*winDivertICMPPacket, bool) {
	pkt := gopacket.NewPacket(raw, packetDecoderForIPVersion(ipVersion), gopacket.NoCopy)
	if ipVersion == 4 {
		return decodeWinDivertICMPv4Packet(pkt, raw)
	}
	return decodeWinDivertICMPv6Packet(pkt, raw)
}

func decodeWinDivertTCPPacket(ipVersion int, raw []byte, dstPort int) (srcPort, seq, ack int, peer net.Addr, ok bool) {
	pkt := gopacket.NewPacket(raw, packetDecoderForIPVersion(ipVersion), gopacket.NoCopy)
	return decodeTCPProbePacket(ipVersion, dstPort, pkt)
}

func decodeWinDivertICMPv4Packet(pkt gopacket.Packet, raw []byte) (*winDivertICMPPacket, bool) {
	ip4, ok := pkt.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
	if !ok || ip4 == nil {
		return nil, false
	}
	ic4, ok := pkt.Layer(layers.LayerTypeICMPv4).(*layers.ICMPv4)
	if !ok || ic4 == nil {
		return nil, false
	}

	packet := &winDivertICMPPacket{
		ipVersion: 4,
		peerIP:    ip4.SrcIP,
		outer:     raw,
	}

	switch ic4.TypeCode.Type() {
	case layers.ICMPv4TypeEchoReply:
		packet.echoReply = true
		packet.echoID = int(ic4.Id)
		packet.echoSeq = int(ic4.Seq)
		return packet, true
	case layers.ICMPv4TypeTimeExceeded, layers.ICMPv4TypeDestinationUnreachable:
		packet.errorData = ic4.Payload
		return packet, true
	default:
		return nil, false
	}
}

func decodeWinDivertICMPv6Packet(pkt gopacket.Packet, raw []byte) (*winDivertICMPPacket, bool) {
	ip6, ok := pkt.Layer(layers.LayerTypeIPv6).(*layers.IPv6)
	if !ok || ip6 == nil {
		return nil, false
	}
	ic6, ok := pkt.Layer(layers.LayerTypeICMPv6).(*layers.ICMPv6)
	if !ok || ic6 == nil || len(ic6.Payload) < 4 {
		return nil, false
	}

	packet := &winDivertICMPPacket{
		ipVersion: 6,
		peerIP:    ip6.SrcIP,
		outer:     raw,
	}

	switch ic6.TypeCode.Type() {
	case layers.ICMPv6TypeEchoReply:
		echo, ok := pkt.Layer(layers.LayerTypeICMPv6Echo).(*layers.ICMPv6Echo)
		if !ok || echo == nil {
			return nil, false
		}
		packet.echoReply = true
		packet.echoID = int(echo.Identifier)
		packet.echoSeq = int(echo.SeqNumber)
		return packet, true
	case layers.ICMPv6TypeTimeExceeded, layers.ICMPv6TypePacketTooBig, layers.ICMPv6TypeDestinationUnreachable:
		packet.errorData = ic6.Payload[4:]
		return packet, true
	default:
		return nil, false
	}
}

func (p *winDivertICMPPacket) message() ReceivedMessage {
	return ReceivedMessage{
		Peer: &net.IPAddr{IP: p.peerIP},
		Msg:  p.outer,
	}
}

func (p *winDivertICMPPacket) echoReplyFor(dstIP net.IP, echoID int) (int, bool) {
	if !p.echoReply || !p.peerIP.Equal(dstIP) || p.echoID != echoID {
		return 0, false
	}
	return p.echoSeq, true
}

func (p *winDivertICMPPacket) errorPayloadFor(dstIP net.IP) ([]byte, bool) {
	if p.echoReply || !matchesEmbeddedDstIP(p.ipVersion, p.errorData, dstIP) {
		return nil, false
	}
	return p.errorData, true
}
