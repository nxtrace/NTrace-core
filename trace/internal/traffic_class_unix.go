//go:build !darwin && !(windows && amd64)

package internal

import (
	"runtime"

	"golang.org/x/net/ipv6"
)

func setIPv6TrafficClass(conn *ipv6.PacketConn, trafficClass uint8) error {
	if runtime.GOOS == "windows" && trafficClass == 0 {
		return nil
	}
	return conn.SetTrafficClass(int(trafficClass))
}
