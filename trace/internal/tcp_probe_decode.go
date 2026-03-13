package internal

import (
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func tcpIPVersionPrefix(ipVersion int) string {
	if ipVersion == 6 {
		return "ip6"
	}
	return "ip"
}

func tcpProbeSequence(tcp *layers.TCP, pktSize int) (int, bool) {
	if tcp == nil {
		return 0, false
	}
	if tcp.ACK && tcp.RST {
		return int(tcp.Ack) - 1 - pktSize, true
	}
	if tcp.ACK && tcp.SYN {
		return int(tcp.Ack) - 1, true
	}
	return 0, false
}

func tcpProbePeerIP(ipVersion int, pkt gopacket.Packet) (net.IP, bool) {
	if ipVersion == 4 {
		ip4, ok := pkt.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
		if !ok || ip4 == nil {
			return nil, false
		}
		return ip4.SrcIP, true
	}

	ip6, ok := pkt.Layer(layers.LayerTypeIPv6).(*layers.IPv6)
	if !ok || ip6 == nil {
		return nil, false
	}
	return ip6.SrcIP, true
}

func decodeTCPProbePacket(ipVersion, dstPort, pktSize int, pkt gopacket.Packet) (srcPort, seq int, peer net.Addr, ok bool) {
	tcp, ok := pkt.Layer(layers.LayerTypeTCP).(*layers.TCP)
	if !ok || tcp == nil || int(tcp.SrcPort) != dstPort {
		return 0, 0, nil, false
	}

	seq, ok = tcpProbeSequence(tcp, pktSize)
	if !ok {
		return 0, 0, nil, false
	}

	peerIP, ok := tcpProbePeerIP(ipVersion, pkt)
	if !ok {
		return 0, 0, nil, false
	}

	return int(tcp.DstPort), seq, &net.IPAddr{IP: peerIP}, true
}
