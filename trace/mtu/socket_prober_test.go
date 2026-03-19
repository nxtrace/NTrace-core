package mtu

import "testing"

func TestProbeDstPortHandlesZeroToken(t *testing.T) {
	if got := probeDstPort(33494, 0); got != 33494 {
		t.Fatalf("probeDstPort() = %d, want %d", got, 33494)
	}
}
