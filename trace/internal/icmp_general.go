//go:build !darwin

package internal

import "net"

func ListenPacket(network string, laddr string) (net.PacketConn, error) {
	return net.ListenPacket(network, laddr)
}
