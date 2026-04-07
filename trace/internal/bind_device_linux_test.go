//go:build linux

package internal

import (
	"net"
	"strings"
	"testing"
)

type fakePacketConn struct {
	net.PacketConn
}

func TestBindPacketConnToSourceDeviceLinuxAllowsEmptyDevice(t *testing.T) {
	if err := bindPacketConnToSourceDevice(nil, 4, ""); err != nil {
		t.Fatalf("bindPacketConnToSourceDevice() error = %v, want nil when device is empty", err)
	}
}

func TestBindPacketConnToSourceDeviceLinuxRejectsNilConn(t *testing.T) {
	err := bindPacketConnToSourceDevice(nil, 4, "eth0")
	if err == nil {
		t.Fatal("bindPacketConnToSourceDevice() error = nil, want non-nil")
	}
	want := `nil PacketConn while binding to device "eth0"`
	if err.Error() != want {
		t.Fatalf("bindPacketConnToSourceDevice() error = %q, want %q", err.Error(), want)
	}
}

func TestBindPacketConnToSourceDeviceLinuxRejectsNonSyscallConn(t *testing.T) {
	err := bindPacketConnToSourceDevice(&fakePacketConn{}, 4, "eth0")
	if err == nil {
		t.Fatal("bindPacketConnToSourceDevice() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "does not support syscall.Conn") {
		t.Fatalf("bindPacketConnToSourceDevice() error = %q, want syscall.Conn rejection", err.Error())
	}
}
