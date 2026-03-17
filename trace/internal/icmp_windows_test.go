//go:build windows && amd64

package internal

import (
	"testing"

	"github.com/google/gopacket/layers"
)

func TestShouldUseICMPv6RawSend(t *testing.T) {
	if shouldUseICMPv6RawSend(nil) {
		t.Fatal("nil header should not use raw send")
	}
	if shouldUseICMPv6RawSend(&layers.IPv6{}) {
		t.Fatal("zero traffic class should keep socket send")
	}
	if !shouldUseICMPv6RawSend(&layers.IPv6{TrafficClass: 46}) {
		t.Fatal("non-zero traffic class should use raw send")
	}
}
