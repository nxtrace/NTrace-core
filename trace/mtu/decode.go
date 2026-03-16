package mtu

import (
	"encoding/binary"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const probePayloadMinLen = 8

type probeResponse struct {
	Event Event
	IP    net.IP
	RTT   time.Duration
	PMTU  int
}

func buildProbePayload(size int) []byte {
	if size < probePayloadMinLen {
		size = probePayloadMinLen
	}
	return make([]byte, size)
}

func parseICMPProbeResult(ipVersion int, raw []byte, peerIP, dstIP net.IP, dstPort, srcPort int) (probeResponse, bool) {
	protocol := 1
	if ipVersion == 6 {
		protocol = 58
	}
	rm, err := icmp.ParseMessage(protocol, raw)
	if err != nil {
		return probeResponse{}, false
	}

	var (
		event Event
		data  []byte
		pmtu  int
	)

	switch ipVersion {
	case 4:
		switch rm.Type {
		case ipv4.ICMPTypeTimeExceeded:
			body, ok := rm.Body.(*icmp.TimeExceeded)
			if !ok || body == nil {
				return probeResponse{}, false
			}
			event = EventTimeExceeded
			data = body.Data
		case ipv4.ICMPTypeDestinationUnreachable:
			body, ok := rm.Body.(*icmp.DstUnreach)
			if !ok || body == nil {
				return probeResponse{}, false
			}
			data = body.Data
			if len(raw) >= 8 && raw[1] == 4 {
				event = EventFragNeeded
				pmtu = int(binary.BigEndian.Uint16(raw[6:8]))
			} else if peerIP != nil && peerIP.Equal(dstIP) {
				event = EventDestination
			} else {
				return probeResponse{}, false
			}
		default:
			return probeResponse{}, false
		}
	case 6:
		switch rm.Type {
		case ipv6.ICMPTypeTimeExceeded:
			body, ok := rm.Body.(*icmp.TimeExceeded)
			if !ok || body == nil {
				return probeResponse{}, false
			}
			event = EventTimeExceeded
			data = body.Data
		case ipv6.ICMPTypePacketTooBig:
			body, ok := rm.Body.(*icmp.PacketTooBig)
			if !ok || body == nil {
				return probeResponse{}, false
			}
			event = EventPacketTooBig
			data = body.Data
			pmtu = body.MTU
		case ipv6.ICMPTypeDestinationUnreachable:
			body, ok := rm.Body.(*icmp.DstUnreach)
			if !ok || body == nil {
				return probeResponse{}, false
			}
			if peerIP == nil || !peerIP.Equal(dstIP) {
				return probeResponse{}, false
			}
			event = EventDestination
			data = body.Data
		default:
			return probeResponse{}, false
		}
	default:
		return probeResponse{}, false
	}

	if !matchesEmbeddedUDP(data, ipVersion, dstIP, dstPort, srcPort) {
		return probeResponse{}, false
	}

	return probeResponse{
		Event: event,
		IP:    peerIP,
		PMTU:  pmtu,
	}, true
}

func matchesEmbeddedUDP(data []byte, ipVersion int, dstIP net.IP, dstPort, srcPort int) bool {
	packet, ok := parseEmbeddedUDPPacket(data, ipVersion)
	if !ok {
		return false
	}
	return packet.dstIP.Equal(dstIP) &&
		packet.srcPort == srcPort &&
		packet.dstPort == dstPort
}

type embeddedUDPPacket struct {
	dstIP   net.IP
	srcPort int
	dstPort int
}

func parseEmbeddedUDPPacket(data []byte, ipVersion int) (embeddedUDPPacket, bool) {
	switch ipVersion {
	case 4:
		if len(data) < 28 || data[0]>>4 != 4 {
			return embeddedUDPPacket{}, false
		}
		ihl := int(data[0]&0x0f) * 4
		if ihl < 20 || len(data) < ihl+8 {
			return embeddedUDPPacket{}, false
		}
		if data[9] != 17 {
			return embeddedUDPPacket{}, false
		}
		return parseEmbeddedUDPFromOffsets(data, ihl, net.IP(data[16:20]))
	case 6:
		if len(data) < 48 || data[0]>>4 != 6 {
			return embeddedUDPPacket{}, false
		}
		if data[6] != 17 {
			return embeddedUDPPacket{}, false
		}
		return parseEmbeddedUDPFromOffsets(data, 40, net.IP(data[24:40]))
	default:
		return embeddedUDPPacket{}, false
	}
}

func parseEmbeddedUDPFromOffsets(data []byte, udpOffset int, dstIP net.IP) (embeddedUDPPacket, bool) {
	if len(data) < udpOffset+8 {
		return embeddedUDPPacket{}, false
	}
	return embeddedUDPPacket{
		dstIP:   append(net.IP(nil), dstIP...),
		srcPort: int(binary.BigEndian.Uint16(data[udpOffset : udpOffset+2])),
		dstPort: int(binary.BigEndian.Uint16(data[udpOffset+2 : udpOffset+4])),
	}, true
}
