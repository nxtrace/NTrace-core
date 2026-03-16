//go:build !linux && !darwin && !windows

package mtu

import "net"

func configurePMTUSocket(_ *net.UDPConn, _ int) error {
	return nil
}

func socketPathMTU(_ *net.UDPConn, _ int) int {
	return 0
}

func isSendSizeErr(_ error) bool {
	return false
}

func isRecvSizeErr(_ error) bool {
	return false
}
