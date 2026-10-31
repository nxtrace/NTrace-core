//go:build windows && !amd64

package internal

import (
	"errors"
	"net"
	"strconv"
	"testing"

	"github.com/google/gopacket/layers"
	"golang.org/x/net/ipv6"
)

func TestSetIPv6TrafficClassSkipsOnlyZeroOnWindows(t *testing.T) {
	conn, err := net.ListenPacket("udp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback is unavailable: %v", err)
	}
	defer conn.Close()
	packetConn := ipv6.NewPacketConn(conn)

	if err := setIPv6TrafficClass(packetConn, 0); err != nil {
		t.Fatalf("zero Traffic Class should not require the unsupported socket option: %v", err)
	}

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := setIPv6TrafficClass(packetConn, 46); err == nil {
		t.Fatal("nonzero Traffic Class unexpectedly bypassed socket setup")
	}
}

func TestIPv6SendersSkipZeroTrafficClassOnWindows(t *testing.T) {
	for _, trafficClass := range []uint8{0, 46} {
		t.Run(strconv.Itoa(int(trafficClass)), func(t *testing.T) {
			ip6 := &layers.IPv6{
				Version:      6,
				TrafficClass: trafficClass,
				HopLimit:     1,
				SrcIP:        net.IPv6loopback,
				DstIP:        net.IPv6loopback,
			}

			t.Run("icmp", func(t *testing.T) {
				conn, packetConn := closedIPv6Socket(t)
				s := ICMPSpec{IPVersion: 6, icmp: conn, icmp6: packetConn}
				_, err := s.SendICMP(t.Context(), ip6,
					&layers.ICMPv6{TypeCode: layers.CreateICMPv6TypeCode(128, 0)},
					&layers.ICMPv6Echo{}, nil)
				assertIPv6TrafficClassInitializationError(t, err, trafficClass != 0)
			})
			t.Run("tcp", func(t *testing.T) {
				conn, packetConn := closedIPv6Socket(t)
				s := TCPSpec{IPVersion: 6, tcp: conn, tcp6: packetConn}
				_, err := s.SendTCP(t.Context(), ip6, &layers.TCP{}, nil)
				assertIPv6TrafficClassInitializationError(t, err, trafficClass != 0)
			})
			t.Run("udp", func(t *testing.T) {
				conn, packetConn := closedIPv6Socket(t)
				s := UDPSpec{IPVersion: 6, udp: conn, udp6: packetConn}
				_, err := s.SendUDP(t.Context(), ip6, &layers.UDP{}, nil)
				assertIPv6TrafficClassInitializationError(t, err, trafficClass != 0)
			})
		})
	}
}

func closedIPv6Socket(t *testing.T) (net.PacketConn, *ipv6.PacketConn) {
	t.Helper()
	conn, err := net.ListenPacket("udp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback is unavailable: %v", err)
	}
	packetConn := ipv6.NewPacketConn(conn)
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	return conn, packetConn
}

func assertIPv6TrafficClassInitializationError(t *testing.T, err error, want bool) {
	t.Helper()
	if err == nil {
		t.Fatal("expected the closed socket to reject the probe")
	}
	var setup *InitializationError
	if got := errors.As(err, &setup); got != want {
		t.Fatalf("Traffic Class initialization error = %v, want %v: %v", got, want, err)
	}
}
