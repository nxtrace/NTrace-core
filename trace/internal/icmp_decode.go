package internal

import (
	"net"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"github.com/nxtrace/NTrace-core/util"
)

func parseSocketICMPMessage(ipVersion int, raw []byte) (*icmp.Message, bool) {
	protocol := 1
	if ipVersion == 6 {
		protocol = 58
	}
	rm, err := icmp.ParseMessage(protocol, raw)
	if err != nil {
		return nil, false
	}
	return rm, true
}

func matchSocketICMPEchoReply(ipVersion int, rm *icmp.Message, peerIP, dstIP net.IP, echoID int) (int, bool) {
	if peerIP == nil || !peerIP.Equal(dstIP) {
		return 0, false
	}
	if !isSocketICMPEchoReply(ipVersion, rm) {
		return 0, false
	}
	echo, ok := rm.Body.(*icmp.Echo)
	if !ok || echo == nil || echo.ID != echoID {
		return 0, false
	}
	return echo.Seq, true
}

func isSocketICMPEchoReply(ipVersion int, rm *icmp.Message) bool {
	switch ipVersion {
	case 4:
		return rm.Type == ipv4.ICMPTypeEchoReply
	case 6:
		return rm.Type == ipv6.ICMPTypeEchoReply
	default:
		return false
	}
}

func extractSocketICMPPayload(ipVersion int, rm *icmp.Message, dstIP net.IP) ([]byte, bool) {
	data, ok := extractSocketICMPErrorBody(ipVersion, rm)
	if !ok || !matchesEmbeddedDstIP(ipVersion, data, dstIP) {
		return nil, false
	}
	return data, true
}

func extractSocketICMPErrorBody(ipVersion int, rm *icmp.Message) ([]byte, bool) {
	switch ipVersion {
	case 4:
		return extractSocketICMPv4Body(rm)
	case 6:
		return extractSocketICMPv6Body(rm)
	default:
		return nil, false
	}
}

func extractSocketICMPv4Body(rm *icmp.Message) ([]byte, bool) {
	switch rm.Type {
	case ipv4.ICMPTypeTimeExceeded:
		body, ok := rm.Body.(*icmp.TimeExceeded)
		return icmpTimeExceededData(body, ok)
	case ipv4.ICMPTypeDestinationUnreachable:
		body, ok := rm.Body.(*icmp.DstUnreach)
		return icmpDstUnreachData(body, ok)
	default:
		return nil, false
	}
}

func extractSocketICMPv6Body(rm *icmp.Message) ([]byte, bool) {
	switch rm.Type {
	case ipv6.ICMPTypeTimeExceeded:
		body, ok := rm.Body.(*icmp.TimeExceeded)
		return icmpTimeExceededData(body, ok)
	case ipv6.ICMPTypePacketTooBig:
		body, ok := rm.Body.(*icmp.PacketTooBig)
		return icmpPacketTooBigData(body, ok)
	case ipv6.ICMPTypeDestinationUnreachable:
		body, ok := rm.Body.(*icmp.DstUnreach)
		return icmpDstUnreachData(body, ok)
	default:
		return nil, false
	}
}

func icmpTimeExceededData(body *icmp.TimeExceeded, ok bool) ([]byte, bool) {
	if !ok || body == nil {
		return nil, false
	}
	return body.Data, true
}

func icmpDstUnreachData(body *icmp.DstUnreach, ok bool) ([]byte, bool) {
	if !ok || body == nil {
		return nil, false
	}
	return body.Data, true
}

func icmpPacketTooBigData(body *icmp.PacketTooBig, ok bool) ([]byte, bool) {
	if !ok || body == nil {
		return nil, false
	}
	return body.Data, true
}

func matchesEmbeddedDstIP(ipVersion int, data []byte, dstIP net.IP) bool {
	embeddedDstIP, ok := extractEmbeddedDstIP(ipVersion, data)
	if !ok {
		return false
	}
	return embeddedDstIP.Equal(dstIP)
}

func extractEmbeddedDstIP(ipVersion int, data []byte) (net.IP, bool) {
	switch ipVersion {
	case 4:
		if len(data) < 20 || data[0]>>4 != 4 {
			return nil, false
		}
		return net.IP(data[16:20]), true
	case 6:
		if len(data) < 40 || data[0]>>4 != 6 {
			return nil, false
		}
		return net.IP(data[24:40]), true
	default:
		return nil, false
	}
}

func extractEmbeddedICMPSeq(data []byte, echoID int) (int, bool) {
	header, err := util.GetICMPResponsePayload(data)
	if err != nil {
		return 0, false
	}
	id, err := util.GetICMPID(header)
	if err != nil || id != echoID {
		return 0, false
	}
	seq, err := util.GetICMPSeq(header)
	if err != nil {
		return 0, false
	}
	return seq, true
}
