//go:build darwin

package internal

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
	"unsafe"
)

//go:linkname internetSocket net.internetSocket
func internetSocket(ctx context.Context, net string, laddr, raddr interface{}, sotype, proto int, mode string, ctrlCtxFn func(context.Context, string, string, syscall.RawConn) error) (fd unsafe.Pointer, err error)

//go:linkname newIPConn net.newIPConn
func newIPConn(fd unsafe.Pointer) *net.IPConn

var (
	errUnknownNetwork = errors.New("unknown network type")

	networkMap = map[string]string{
		"ip4:icmp": "udp4",
		"ip4:1":    "udp4",
		"ip6:icmp": "udp6",
		"ip6:58":   "udp6",
	}
)

// ListenICMP 会造成指定出口IP功能不可使用
func ListenICMP(network string, laddr string) (net.PacketConn, error) {
	if os.Getuid() == 0 { // root
		return net.ListenPacket(network, laddr)
	} else {
		if nw, ok := networkMap[network]; ok {
			proto := syscall.IPPROTO_ICMP
			if nw == "udp6" {
				proto = syscall.IPPROTO_ICMPV6
			}
			isock, err := internetSocket(context.Background(), nw, nil, nil, syscall.SOCK_DGRAM, proto, "listen", nil)
			if err != nil {
				panic(err)
			}
			return newIPConn(isock), nil
		} else {
			return nil, errUnknownNetwork
		}
	}
}
