package trace

import (
	"net"
	"testing"
)

func TestNormalizePacketSize(t *testing.T) {
	tests := []struct {
		name       string
		method     Method
		ip         net.IP
		packetSize int
		wantSize   int
		wantRandom bool
	}{
		{name: "icmp4", method: ICMPTrace, ip: net.ParseIP("1.1.1.1"), packetSize: 52, wantSize: 24},
		{name: "udp4", method: UDPTrace, ip: net.ParseIP("1.1.1.1"), packetSize: 52, wantSize: 24},
		{name: "icmp6", method: ICMPTrace, ip: net.ParseIP("2001:db8::1"), packetSize: 64, wantSize: 16},
		{name: "udp6", method: UDPTrace, ip: net.ParseIP("2001:db8::1"), packetSize: 64, wantSize: 16},
		{name: "tcp4", method: TCPTrace, ip: net.ParseIP("1.1.1.1"), packetSize: 64, wantSize: 20},
		{name: "tcp6-random", method: TCPTrace, ip: net.ParseIP("2001:db8::1"), packetSize: -96, wantSize: 32, wantRandom: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := NormalizePacketSize(tt.method, tt.ip, tt.packetSize)
			if err != nil {
				t.Fatalf("NormalizePacketSize() error = %v", err)
			}
			if spec.PayloadSize != tt.wantSize {
				t.Fatalf("PayloadSize = %d, want %d", spec.PayloadSize, tt.wantSize)
			}
			if spec.Random != tt.wantRandom {
				t.Fatalf("Random = %v, want %v", spec.Random, tt.wantRandom)
			}
		})
	}
}

func TestNormalizePacketSizeRejectsTooSmallValues(t *testing.T) {
	tests := []struct {
		name       string
		method     Method
		ip         net.IP
		packetSize int
	}{
		{name: "icmp4", method: ICMPTrace, ip: net.ParseIP("1.1.1.1"), packetSize: 27},
		{name: "icmp4-zero", method: ICMPTrace, ip: net.ParseIP("1.1.1.1"), packetSize: 0},
		{name: "icmp6", method: ICMPTrace, ip: net.ParseIP("2001:db8::1"), packetSize: 47},
		{name: "udp6", method: UDPTrace, ip: net.ParseIP("2001:db8::1"), packetSize: 49},
		{name: "tcp4", method: TCPTrace, ip: net.ParseIP("1.1.1.1"), packetSize: 43},
		{name: "tcp6", method: TCPTrace, ip: net.ParseIP("2001:db8::1"), packetSize: 63},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NormalizePacketSize(tt.method, tt.ip, tt.packetSize); err == nil {
				t.Fatal("NormalizePacketSize() error = nil, want error")
			}
		})
	}
}

func TestMinPacketSize(t *testing.T) {
	tests := []struct {
		method Method
		ip     net.IP
		want   int
	}{
		{method: ICMPTrace, ip: net.ParseIP("1.1.1.1"), want: 28},
		{method: UDPTrace, ip: net.ParseIP("1.1.1.1"), want: 28},
		{method: ICMPTrace, ip: net.ParseIP("2001:db8::1"), want: 48},
		{method: UDPTrace, ip: net.ParseIP("2001:db8::1"), want: 50},
		{method: TCPTrace, ip: net.ParseIP("1.1.1.1"), want: 44},
		{method: TCPTrace, ip: net.ParseIP("2001:db8::1"), want: 64},
	}

	for _, tt := range tests {
		if got := MinPacketSize(tt.method, tt.ip); got != tt.want {
			t.Fatalf("MinPacketSize(%s, %v) = %d, want %d", tt.method, tt.ip, got, tt.want)
		}
	}
}
