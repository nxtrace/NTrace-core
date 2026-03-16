//go:build linux

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
			controlErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_MTU_DISCOVER, unix.IPV6_PMTUDISC_DO)
			return
		}
		controlErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_MTU_DISCOVER, unix.IP_PMTUDISC_DO)
	}); err != nil {
		return err
	}
	return controlErr
}

func socketPathMTU(conn *net.UDPConn, ipVersion int) int {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return 0
	}
	mtu := 0
	_ = rawConn.Control(func(fd uintptr) {
		if ipVersion == 6 {
			mtu, _ = unix.GetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_MTU)
			return
		}
		mtu, _ = unix.GetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_MTU)
	})
	return mtu
}

func isSendSizeErr(err error) bool {
	return errors.Is(err, unix.EMSGSIZE)
}

func isRecvSizeErr(err error) bool {
	return errors.Is(err, unix.EMSGSIZE)
}
