package internal

import (
	"errors"
	"net"
	"os"
	"runtime"

	"golang.org/x/net/icmp"
)

var (
	errRootRequired   = errors.New("root permission required to use ICMP trace on this platform")
	errUnknownNetwork = errors.New("unknown network type")

	networkMap = map[string]string{
		"ip4:icmp": "udp4",
		"ip4:1":    "udp4",
		"ip6:icmp": "udp6",
		"ip6:58":   "udp6",
	}
)

func ListenICMP(network string, laddr string) (net.PacketConn, error) {
	if os.Getuid() == 0 { // root
		return net.ListenPacket(network, laddr)
	} else if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if nw, ok := networkMap[network]; ok {
			return icmp.ListenPacket(nw, laddr)
		} else {
			return nil, errUnknownNetwork
		}
	} else {
		return nil, errRootRequired
	}
}
