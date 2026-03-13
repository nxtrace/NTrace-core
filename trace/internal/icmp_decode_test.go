package internal

import (
	"encoding/binary"
	"net"
	"testing"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func mustMarshalICMP(t *testing.T, message icmp.Message) []byte {
	t.Helper()
	raw, err := message.Marshal(nil)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return raw
}

func buildIPv4InnerPacket(dstIP net.IP, echoID, seq int) []byte {
	packet := make([]byte, 28)
	packet[0] = 0x45
	copy(packet[16:20], dstIP.To4())
	binary.BigEndian.PutUint16(packet[24:26], uint16(echoID))
	binary.BigEndian.PutUint16(packet[26:28], uint16(seq))
	return packet
}

func buildIPv6InnerPacket(dstIP net.IP, echoID, seq int) []byte {
	packet := make([]byte, 48)
	packet[0] = 0x60
	packet[6] = 58
	copy(packet[24:40], dstIP.To16())
	binary.BigEndian.PutUint16(packet[44:46], uint16(echoID))
	binary.BigEndian.PutUint16(packet[46:48], uint16(seq))
	return packet
}

func TestMatchSocketICMPEchoReplyIPv4(t *testing.T) {
	dstIP := net.ParseIP("1.1.1.1")
	raw := mustMarshalICMP(t, icmp.Message{
		Type: ipv4.ICMPTypeEchoReply,
		Code: 0,
		Body: &icmp.Echo{ID: 7, Seq: 11},
	})

	rm, ok := parseSocketICMPMessage(4, raw)
	if !ok {
		t.Fatalf("parseSocketICMPMessage() ok = false")
	}
	seq, ok := matchSocketICMPEchoReply(4, rm, dstIP, dstIP, 7)
	if !ok || seq != 11 {
		t.Fatalf("matchSocketICMPEchoReply() = (%d, %v), want (11, true)", seq, ok)
	}
}

func TestMatchSocketICMPEchoReplyIPv6(t *testing.T) {
	dstIP := net.ParseIP("2001:db8::1")
	raw := mustMarshalICMP(t, icmp.Message{
		Type: ipv6.ICMPTypeEchoReply,
		Code: 0,
		Body: &icmp.Echo{ID: 9, Seq: 21},
	})

	rm, ok := parseSocketICMPMessage(6, raw)
	if !ok {
		t.Fatalf("parseSocketICMPMessage() ok = false")
	}
	seq, ok := matchSocketICMPEchoReply(6, rm, dstIP, dstIP, 9)
	if !ok || seq != 21 {
		t.Fatalf("matchSocketICMPEchoReply() = (%d, %v), want (21, true)", seq, ok)
	}
}

func TestExtractSocketICMPPayloadIPv4(t *testing.T) {
	dstIP := net.ParseIP("8.8.8.8")
	inner := buildIPv4InnerPacket(dstIP, 13, 99)
	raw := mustMarshalICMP(t, icmp.Message{
		Type: ipv4.ICMPTypeTimeExceeded,
		Code: 0,
		Body: &icmp.TimeExceeded{Data: inner},
	})

	rm, ok := parseSocketICMPMessage(4, raw)
	if !ok {
		t.Fatalf("parseSocketICMPMessage() ok = false")
	}
	data, ok := extractSocketICMPPayload(4, rm, dstIP)
	if !ok {
		t.Fatalf("extractSocketICMPPayload() ok = false")
	}
	if seq, ok := extractEmbeddedICMPSeq(data, 13); !ok || seq != 99 {
		t.Fatalf("extractEmbeddedICMPSeq() = (%d, %v), want (99, true)", seq, ok)
	}
}

func TestExtractSocketICMPPayloadIPv6(t *testing.T) {
	dstIP := net.ParseIP("2001:db8::2")
	inner := buildIPv6InnerPacket(dstIP, 17, 123)
	raw := mustMarshalICMP(t, icmp.Message{
		Type: ipv6.ICMPTypeDestinationUnreachable,
		Code: 0,
		Body: &icmp.DstUnreach{Data: inner},
	})

	rm, ok := parseSocketICMPMessage(6, raw)
	if !ok {
		t.Fatalf("parseSocketICMPMessage() ok = false")
	}
	data, ok := extractSocketICMPPayload(6, rm, dstIP)
	if !ok {
		t.Fatalf("extractSocketICMPPayload() ok = false")
	}
	if seq, ok := extractEmbeddedICMPSeq(data, 17); !ok || seq != 123 {
		t.Fatalf("extractEmbeddedICMPSeq() = (%d, %v), want (123, true)", seq, ok)
	}
}

func TestExtractSocketICMPPayloadRejectsWrongDestination(t *testing.T) {
	raw := mustMarshalICMP(t, icmp.Message{
		Type: ipv4.ICMPTypeDestinationUnreachable,
		Code: 0,
		Body: &icmp.DstUnreach{Data: buildIPv4InnerPacket(net.ParseIP("9.9.9.9"), 3, 5)},
	})

	rm, ok := parseSocketICMPMessage(4, raw)
	if !ok {
		t.Fatalf("parseSocketICMPMessage() ok = false")
	}
	if _, ok := extractSocketICMPPayload(4, rm, net.ParseIP("8.8.8.8")); ok {
		t.Fatalf("extractSocketICMPPayload() ok = true, want false")
	}
}
