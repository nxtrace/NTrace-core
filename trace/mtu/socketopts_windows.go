//go:build windows

package mtu

import (
	"errors"
	"net"

	"github.com/nxtrace/NTrace-core/util"
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
			controlErr = windows.SetsockoptInt(
				windows.Handle(fd),
				windows.IPPROTO_IPV6,
				windows.IPV6_MTU_DISCOVER,
				windows.IP_PMTUDISC_PROBE,
			)
			if controlErr != nil {
				controlErr = windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IPV6, ipv6DontFrag, 1)
			}
			return
		}
		controlErr = windows.SetsockoptInt(
			windows.Handle(fd),
			windows.IPPROTO_IP,
			windows.IP_MTU_DISCOVER,
			windows.IP_PMTUDISC_PROBE,
		)
		if controlErr != nil {
			controlErr = windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IP, ipDontFragment, 1)
		}
	}); err != nil {
		return err
	}
	return controlErr
}

func socketPathMTU(conn *net.UDPConn, _ int) int {
	if conn == nil {
		return 0
	}
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr == nil || addr.IP == nil {
		return 0
	}
	return util.GetMTUByIPForDevice(addr.IP, "")
}

func isSendSizeErr(err error) bool {
	return errors.Is(err, windows.WSAEMSGSIZE)
}

func isRecvSizeErr(err error) bool {
	return errors.Is(err, windows.WSAEMSGSIZE)
}
