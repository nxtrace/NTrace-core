//go:build !darwin && !windows

package internal

import "golang.org/x/net/ipv6"

func setIPv6TrafficClass(conn *ipv6.PacketConn, trafficClass uint8) error {
	return conn.SetTrafficClass(int(trafficClass))
}
