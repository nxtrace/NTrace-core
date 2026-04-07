//go:build linux

package internal

import (
	"fmt"
	"net"
	"syscall"
)

func bindPacketConnToSourceDevice(conn net.PacketConn, ipVersion int, device string) error {
	_ = ipVersion
	if conn == nil || device == "" {
		return nil
	}

	sysconn, ok := conn.(syscall.Conn)
	if !ok {
		return fmt.Errorf("packet conn does not support syscall.Conn")
	}
	rawConn, err := sysconn.SyscallConn()
	if err != nil {
		return fmt.Errorf("packet conn SyscallConn: %w", err)
	}

	var controlErr error
	if err := rawConn.Control(func(fd uintptr) {
		controlErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, device)
	}); err != nil {
		return fmt.Errorf("packet conn control: %w", err)
	}
	if controlErr != nil {
		return fmt.Errorf("bind packet conn to device %q: %w", device, controlErr)
	}
	return nil
}
