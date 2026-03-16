//go:build windows

package mtu

import (
	"errors"
	"net"

	"golang.org/x/sys/windows"
)

const (
	ipDontFragment = 14
	ipv6DontFrag   = 14
)

func configurePMTUSocket(conn *net.UDPConn, ipVersion int) error {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var controlErr error
	if err := rawConn.Control(func(fd uintptr) {
		if ipVersion == 6 {
			controlErr = windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IPV6, ipv6DontFrag, 1)
			return
		}
		controlErr = windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IP, ipDontFragment, 1)
	}); err != nil {
		return err
	}
	return controlErr
}

func socketPathMTU(_ *net.UDPConn, _ int) int {
	return 0
}

func isSendSizeErr(err error) bool {
	return errors.Is(err, windows.WSAEMSGSIZE)
}

func isRecvSizeErr(err error) bool {
	return errors.Is(err, windows.WSAEMSGSIZE)
}
