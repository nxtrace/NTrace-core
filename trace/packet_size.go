package trace

import (
	"fmt"
	"math/rand"
	"net"

	"github.com/nxtrace/NTrace-core/util"
)

const (
	ipv4HeaderBytes     = 20
	ipv6HeaderBytes     = 40
	icmpHeaderBytes     = 8
	udpHeaderBytes      = 8
	tcpProbeHeaderBytes = 24
	udpV6MinPayload     = 2
)

type PacketSizeSpec struct {
	PayloadSize int
	Random      bool
}

func packetSizeIPHeaderBytes(dstIP net.IP) int {
	if util.IsIPv6(dstIP) {
		return ipv6HeaderBytes
	}
	return ipv4HeaderBytes
}

func packetSizeProtocolHeaderBytes(method Method) int {
	switch method {
	case TCPTrace:
		return tcpProbeHeaderBytes
	case UDPTrace:
		return udpHeaderBytes
	default:
		return icmpHeaderBytes
	}
}

func packetSizeMinPayload(method Method, dstIP net.IP) int {
	if method == UDPTrace && util.IsIPv6(dstIP) {
		return udpV6MinPayload
	}
	return 0
}

func MinPacketSize(method Method, dstIP net.IP) int {
	return packetSizeIPHeaderBytes(dstIP) + packetSizeProtocolHeaderBytes(method) + packetSizeMinPayload(method, dstIP)
}

func NormalizePacketSize(method Method, dstIP net.IP, packetSize int) (PacketSizeSpec, error) {
	random := packetSize < 0
	packetSizeAbs := packetSize
	if random {
		packetSizeAbs = -packetSizeAbs
	}

	minSize := MinPacketSize(method, dstIP)
	if packetSizeAbs < minSize {
		return PacketSizeSpec{}, fmt.Errorf("packet size %d is too small for %s over %s; minimum is %d", packetSize, method, packetSizeFamilyLabel(dstIP), minSize)
	}

	payloadSize := packetSizeAbs - packetSizeIPHeaderBytes(dstIP) - packetSizeProtocolHeaderBytes(method)
	if payloadSize < packetSizeMinPayload(method, dstIP) {
		return PacketSizeSpec{}, fmt.Errorf("packet size %d is too small for %s over %s; minimum is %d", packetSize, method, packetSizeFamilyLabel(dstIP), minSize)
	}

	return PacketSizeSpec{
		PayloadSize: payloadSize,
		Random:      random,
	}, nil
}

func resolveProbePayloadSize(method Method, dstIP net.IP, maxPayloadSize int, randomPerProbe bool) int {
	minPayload := packetSizeMinPayload(method, dstIP)
	if !randomPerProbe || maxPayloadSize <= minPayload {
		return maxPayloadSize
	}
	return minPayload + rand.Intn(maxPayloadSize-minPayload+1)
}

func packetSizeFamilyLabel(dstIP net.IP) string {
	if util.IsIPv6(dstIP) {
		return "IPv6"
	}
	return "IPv4"
}

func FormatPacketSizeLabel(packetSize int) string {
	if packetSize < 0 {
		return fmt.Sprintf("random <= %d byte packets", -packetSize)
	}
	return fmt.Sprintf("%d byte packets", packetSize)
}
