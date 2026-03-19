package mtu

import (
	"net"
	"testing"
)

func TestProbeDstPortHandlesZeroToken(t *testing.T) {
	if got := probeDstPort(33494, 0); got != 33494 {
		t.Fatalf("probeDstPort() = %d, want %d", got, 33494)
	}
}

func TestBuildWinDivertMTUFilter(t *testing.T) {
	tests := []struct {
		name      string
		ipVersion int
		srcIP     net.IP
		want      string
	}{
		{
			name:      "ipv4 nil source",
			ipVersion: 4,
			srcIP:     nil,
			want:      "inbound and icmp",
		},
		{
			name:      "ipv4 unspecified source",
			ipVersion: 4,
			srcIP:     net.IPv4zero,
			want:      "inbound and icmp",
		},
		{
			name:      "ipv4 specified source",
			ipVersion: 4,
			srcIP:     net.ParseIP("192.0.2.10"),
			want:      "inbound and icmp and ip.DstAddr == 192.0.2.10",
		},
		{
			name:      "ipv6 nil source",
			ipVersion: 6,
			srcIP:     nil,
			want:      "inbound and icmpv6",
		},
		{
			name:      "ipv6 unspecified source",
			ipVersion: 6,
			srcIP:     net.IPv6zero,
			want:      "inbound and icmpv6",
		},
		{
			name:      "ipv6 specified source",
			ipVersion: 6,
			srcIP:     net.ParseIP("2001:db8::10"),
			want:      "inbound and icmpv6 and ipv6.DstAddr == 2001:db8::10",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildWinDivertMTUFilter(tc.ipVersion, tc.srcIP); got != tc.want {
				t.Fatalf("buildWinDivertMTUFilter() = %q, want %q", got, tc.want)
			}
		})
	}
}
