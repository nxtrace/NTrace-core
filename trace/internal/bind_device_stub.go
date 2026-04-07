//go:build !darwin && !linux

package internal

import "net"

func bindPacketConnToSourceDevice(conn net.PacketConn, ipVersion int, device string) error {
	_ = conn
	_ = ipVersion
	_ = device
	return nil
}
