package mtu

import (
	"encoding/binary"
	"net"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func TestParseICMPProbeResultIPv4FragNeeded(t *testing.T) {
	dstIP := net.ParseIP("203.0.113.9")
	peerIP := net.ParseIP("198.51.100.1")
	inner := mustSerializeIPv4UDP(t, net.ParseIP("192.0.2.10"), dstIP, 40000, 33494, buildProbePayload(64))

	msg := icmp.Message{
		Type: ipv4.ICMPTypeDestinationUnreachable,
		Code: 4,
		Body: &icmp.DstUnreach{Data: inner},
	}
	raw, err := msg.Marshal(nil)
	if err != nil {
		t.Fatalf("marshal icmp: %v", err)
	}
	binary.BigEndian.PutUint16(raw[6:8], 1400)

	resp, ok := parseICMPProbeResult(4, raw, peerIP, dstIP, 33494, 40000)
	if !ok {
		t.Fatal("expected frag-needed response to match")
	}
	if resp.Event != EventFragNeeded {
		t.Fatalf("event = %q, want %q", resp.Event, EventFragNeeded)
	}
	if resp.PMTU != 1400 {
		t.Fatalf("pmtu = %d, want 1400", resp.PMTU)
	}
	if !resp.IP.Equal(peerIP) {
		t.Fatalf("peer = %v, want %v", resp.IP, peerIP)
	}
}

func TestParseICMPProbeResultIPv6PacketTooBig(t *testing.T) {
	dstIP := net.ParseIP("2001:db8::9")
	peerIP := net.ParseIP("2001:db8::1")
	inner := mustSerializeIPv6UDP(t, net.ParseIP("2001:db8::10"), dstIP, 40001, 33494, buildProbePayload(80))

	msg := icmp.Message{
		Type: ipv6.ICMPTypePacketTooBig,
		Code: 0,
		Body: &icmp.PacketTooBig{MTU: 1280, Data: inner},
	}
	raw, err := msg.Marshal(nil)
	if err != nil {
		t.Fatalf("marshal icmpv6: %v", err)
	}

	resp, ok := parseICMPProbeResult(6, raw, peerIP, dstIP, 33494, 40001)
	if !ok {
		t.Fatal("expected packet-too-big response to match")
	}
	if resp.Event != EventPacketTooBig {
		t.Fatalf("event = %q, want %q", resp.Event, EventPacketTooBig)
	}
	if resp.PMTU != 1280 {
		t.Fatalf("pmtu = %d, want 1280", resp.PMTU)
	}
}

func TestParseICMPProbeResultIPv4MatchesMinimumQuotedUDPHeader(t *testing.T) {
	dstIP := net.ParseIP("203.0.113.9")
	peerIP := net.ParseIP("198.51.100.1")
	inner := mustSerializeIPv4UDP(t, net.ParseIP("192.0.2.10"), dstIP, 40000, 33494, nil)
	inner = inner[:28]

	msg := icmp.Message{
		Type: ipv4.ICMPTypeDestinationUnreachable,
		Code: 4,
		Body: &icmp.DstUnreach{Data: inner},
	}
	raw, err := msg.Marshal(nil)
	if err != nil {
		t.Fatalf("marshal icmp: %v", err)
	}
	binary.BigEndian.PutUint16(raw[6:8], 1500)

	resp, ok := parseICMPProbeResult(4, raw, peerIP, dstIP, 33494, 40000)
	if !ok {
		t.Fatal("expected minimal quoted udp header to match")
	}
	if resp.PMTU != 1500 {
		t.Fatalf("pmtu = %d, want 1500", resp.PMTU)
	}
}

func TestParseEmbeddedUDPPacketIPv6WithExtensionHeaders(t *testing.T) {
	dstIP := net.ParseIP("2001:db8::9")
	data := make([]byte, 56)
	data[0] = 6 << 4
	data[6] = 0
	copy(data[24:40], dstIP.To16())

	data[40] = 17
	data[41] = 0
	binary.BigEndian.PutUint16(data[48:50], 40001)
	binary.BigEndian.PutUint16(data[50:52], 33494)

	packet, ok := parseEmbeddedUDPPacket(data, 6)
	if !ok {
		t.Fatal("expected IPv6 UDP packet behind extension header to match")
	}
	if !packet.dstIP.Equal(dstIP) {
		t.Fatalf("dst ip = %v, want %v", packet.dstIP, dstIP)
	}
	if packet.srcPort != 40001 || packet.dstPort != 33494 {
		t.Fatalf("unexpected ports: %+v", packet)
	}
}

func mustSerializeIPv4UDP(t *testing.T, srcIP, dstIP net.IP, srcPort, dstPort int, payload []byte) []byte {
	t.Helper()
	ip := &layers.IPv4{
		Version:  4,
		TTL:      1,
		SrcIP:    srcIP.To4(),
		DstIP:    dstIP.To4(),
		Protocol: layers.IPProtocolUDP,
	}
	udp := &layers.UDP{
		SrcPort: layers.UDPPort(srcPort),
		DstPort: layers.UDPPort(dstPort),
	}
	if err := udp.SetNetworkLayerForChecksum(ip); err != nil {
		t.Fatalf("set checksum: %v", err)
	}
	return mustSerializeLayers(t, ip, udp, gopacket.Payload(payload))
}

func mustSerializeIPv6UDP(t *testing.T, srcIP, dstIP net.IP, srcPort, dstPort int, payload []byte) []byte {
	t.Helper()
	ip := &layers.IPv6{
		Version:    6,
		HopLimit:   1,
		SrcIP:      srcIP,
		DstIP:      dstIP,
		NextHeader: layers.IPProtocolUDP,
	}
	udp := &layers.UDP{
		SrcPort: layers.UDPPort(srcPort),
		DstPort: layers.UDPPort(dstPort),
	}
	if err := udp.SetNetworkLayerForChecksum(ip); err != nil {
		t.Fatalf("set checksum: %v", err)
	}
	return mustSerializeLayers(t, ip, udp, gopacket.Payload(payload))
}

func mustSerializeLayers(t *testing.T, layersToSerialize ...gopacket.SerializableLayer) []byte {
	t.Helper()
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{ComputeChecksums: true, FixLengths: true}
	if err := gopacket.SerializeLayers(buf, opts, layersToSerialize...); err != nil {
		t.Fatalf("serialize layers: %v", err)
	}
	return buf.Bytes()
}
