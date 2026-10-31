//go:build windows && !amd64

package internal

import "golang.org/x/net/ipv6"

func setIPv6TrafficClass(conn *ipv6.PacketConn, trafficClass uint8) error {
	if trafficClass == 0 {
		return nil
	}
	return conn.SetTrafficClass(int(trafficClass))
}
