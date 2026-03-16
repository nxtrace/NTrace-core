//go:build darwin

package mtu

import (
	"errors"
	"net"

	"golang.org/x/sys/unix"
)

func configurePMTUSocket(conn *net.UDPConn, ipVersion int) error {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var controlErr error
	if err := rawConn.Control(func(fd uintptr) {
		if ipVersion == 6 {
			controlErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_DONTFRAG, 1)
			return
		}
		controlErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_DONTFRAG, 1)
	}); err != nil {
		return err
	}
	return controlErr
}

func socketPathMTU(_ *net.UDPConn, _ int) int {
	return 0
}

func isSendSizeErr(err error) bool {
	return errors.Is(err, unix.EMSGSIZE)
}

func isRecvSizeErr(err error) bool {
	return errors.Is(err, unix.EMSGSIZE)
}
