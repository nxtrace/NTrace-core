//go:build darwin

package internal

import (
	"fmt"
	"net"
	"syscall"
)

func bindPacketConnToSourceDevice(conn net.PacketConn, ipVersion int, device string) error {
	if device == "" {
		return nil
	}
	if conn == nil {
		return fmt.Errorf("nil PacketConn while binding to device %q", device)
	}

	sysconn, ok := conn.(syscall.Conn)
	if !ok {
		return fmt.Errorf("packet conn does not support syscall.Conn")
	}
	rawConn, err := sysconn.SyscallConn()
	if err != nil {
		return fmt.Errorf("packet conn SyscallConn: %w", err)
	}

	iface, err := net.InterfaceByName(device)
	if err != nil {
		return fmt.Errorf("lookup source device %q: %w", device, err)
	}
	if iface == nil {
		return fmt.Errorf("source device %q not found", device)
	}

	level := syscall.IPPROTO_IP
	opt := syscall.IP_BOUND_IF
	if ipVersion == 6 {
		level = syscall.IPPROTO_IPV6
		opt = syscall.IPV6_BOUND_IF
	}

	var controlErr error
	if err := rawConn.Control(func(fd uintptr) {
		controlErr = syscall.SetsockoptInt(int(fd), level, opt, iface.Index)
	}); err != nil {
		return fmt.Errorf("packet conn control: %w", err)
	}
	if controlErr != nil {
		return fmt.Errorf("bind packet conn to device %q: %w", device, controlErr)
	}
	return nil
}
