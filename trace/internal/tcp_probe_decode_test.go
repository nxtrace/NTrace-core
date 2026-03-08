package internal

import (
	"net"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func mustSerializeTCPProbePacket(t *testing.T, ipLayer gopacket.NetworkLayer, tcp *layers.TCP) gopacket.Packet {
	t.Helper()

	if err := tcp.SetNetworkLayerForChecksum(ipLayer); err != nil {
		t.Fatalf("SetNetworkLayerForChecksum() error = %v", err)
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}
	switch ipLayer.(type) {
	case *layers.IPv4:
		if err := gopacket.SerializeLayers(buf, opts, ipLayer.(*layers.IPv4), tcp); err != nil {
			t.Fatalf("SerializeLayers() error = %v", err)
		}
		return gopacket.NewPacket(buf.Bytes(), layers.LayerTypeIPv4, gopacket.NoCopy)
	case *layers.IPv6:
		if err := gopacket.SerializeLayers(buf, opts, ipLayer.(*layers.IPv6), tcp); err != nil {
			t.Fatalf("SerializeLayers() error = %v", err)
		}
		return gopacket.NewPacket(buf.Bytes(), layers.LayerTypeIPv6, gopacket.NoCopy)
	default:
		t.Fatalf("unexpected IP layer type %T", ipLayer)
		return nil
	}
}

func TestDecodeTCPProbePacketIPv4RSTAck(t *testing.T) {
	srcIP := net.ParseIP("1.1.1.1")
	dstIP := net.ParseIP("2.2.2.2")
	ip4 := &layers.IPv4{
		Version:  4,
		IHL:      5,
		Protocol: layers.IPProtocolTCP,
		SrcIP:    srcIP,
		DstIP:    dstIP,
	}
	tcp := &layers.TCP{
		SrcPort: 443,
		DstPort: 32100,
		ACK:     true,
		RST:     true,
		Ack:     200,
	}

	pkt := mustSerializeTCPProbePacket(t, ip4, tcp)
	srcPort, seq, peer, ok := decodeTCPProbePacket(4, 443, 32, pkt)
	if !ok {
		t.Fatalf("decodeTCPProbePacket() ok = false")
	}
	if srcPort != 32100 || seq != 167 {
		t.Fatalf("decodeTCPProbePacket() = (%d, %d), want (32100, 167)", srcPort, seq)
	}
	if got := peer.(*net.IPAddr).IP; !got.Equal(srcIP) {
		t.Fatalf("peer IP = %v, want %v", got, srcIP)
	}
}

func TestDecodeTCPProbePacketIPv6SYNAck(t *testing.T) {
	srcIP := net.ParseIP("2001:db8::1")
	dstIP := net.ParseIP("2001:db8::2")
	ip6 := &layers.IPv6{
		Version:    6,
		NextHeader: layers.IPProtocolTCP,
		SrcIP:      srcIP,
		DstIP:      dstIP,
	}
	tcp := &layers.TCP{
		SrcPort: 8443,
		DstPort: 45678,
		ACK:     true,
		SYN:     true,
		Ack:     91,
	}

	pkt := mustSerializeTCPProbePacket(t, ip6, tcp)
	srcPort, seq, peer, ok := decodeTCPProbePacket(6, 8443, 0, pkt)
	if !ok {
		t.Fatalf("decodeTCPProbePacket() ok = false")
	}
	if srcPort != 45678 || seq != 90 {
		t.Fatalf("decodeTCPProbePacket() = (%d, %d), want (45678, 90)", srcPort, seq)
	}
	if got := peer.(*net.IPAddr).IP; !got.Equal(srcIP) {
		t.Fatalf("peer IP = %v, want %v", got, srcIP)
	}
}

func TestDecodeTCPProbePacketRejectsUnexpectedPort(t *testing.T) {
	ip4 := &layers.IPv4{
		Version:  4,
		IHL:      5,
		Protocol: layers.IPProtocolTCP,
		SrcIP:    net.ParseIP("3.3.3.3"),
		DstIP:    net.ParseIP("4.4.4.4"),
	}
	tcp := &layers.TCP{
		SrcPort: 80,
		DstPort: 50000,
		ACK:     true,
		SYN:     true,
		Ack:     50,
	}

	pkt := mustSerializeTCPProbePacket(t, ip4, tcp)
	if _, _, _, ok := decodeTCPProbePacket(4, 443, 0, pkt); ok {
		t.Fatalf("decodeTCPProbePacket() ok = true, want false")
	}
}
