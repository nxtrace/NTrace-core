//go:build windows || (!darwin && !linux)

package internal

import "testing"

func TestBindPacketConnToSourceDeviceStubAllowsEmptyDevice(t *testing.T) {
	if err := bindPacketConnToSourceDevice(nil, 4, ""); err != nil {
		t.Fatalf("bindPacketConnToSourceDevice() error = %v, want nil when device is empty", err)
	}
}

func TestBindPacketConnToSourceDeviceStubRejectsExplicitDevice(t *testing.T) {
	err := bindPacketConnToSourceDevice(nil, 4, "Ethernet0")
	if err == nil {
		t.Fatal("bindPacketConnToSourceDevice() error = nil, want non-nil")
	}
	want := "binding to source device not supported on this platform: Ethernet0"
	if err.Error() != want {
		t.Fatalf("bindPacketConnToSourceDevice() error = %q, want %q", err.Error(), want)
	}
}
