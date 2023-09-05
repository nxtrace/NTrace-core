//go:build !darwin

package internal

import "net"

func ListenICMP(network string, laddr string) (net.PacketConn, error) {
	return net.ListenPacket(network, laddr)
}
