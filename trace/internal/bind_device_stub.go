//go:build !darwin && !linux

package internal

import (
	"fmt"
	"net"
)

func bindPacketConnToSourceDevice(conn net.PacketConn, ipVersion int, device string) error {
	_ = conn
	_ = ipVersion
	if device != "" {
		return fmt.Errorf("binding to source device not supported on this platform: %s", device)
	}
	return nil
}
