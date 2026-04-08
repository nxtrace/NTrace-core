//go:build darwin

package internal

import "testing"

func TestBindPacketConnToSourceDeviceAllowsEmptyDevice(t *testing.T) {
	if err := bindPacketConnToSourceDevice(nil, 4, ""); err != nil {
		t.Fatalf("bindPacketConnToSourceDevice() error = %v, want nil when device is empty", err)
	}
}

func TestBindPacketConnToSourceDeviceRejectsNilConn(t *testing.T) {
	err := bindPacketConnToSourceDevice(nil, 4, "en0")
	if err == nil {
		t.Fatal("bindPacketConnToSourceDevice() error = nil, want non-nil")
	}
	want := `nil PacketConn while binding to device "en0"`
	if err.Error() != want {
		t.Fatalf("bindPacketConnToSourceDevice() error = %q, want %q", err.Error(), want)
	}
}
