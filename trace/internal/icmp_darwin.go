//go:build darwin

package internal

import (
	"context"
	"errors"
	"net"
	"syscall"
	"unsafe"
)

//go:linkname internetSocket net.internetSocket
func internetSocket(ctx context.Context, net string, laddr, raddr any, sotype, proto int, mode string, ctrlCtxFn func(context.Context, string, string, syscall.RawConn) error) (fd unsafe.Pointer, err error)

//go:linkname newIPConn net.newIPConn
func newIPConn(fd unsafe.Pointer) *net.IPConn

var (
	errUnknownNetwork = errors.New("unknown network type")
	errUnknownIface   = errors.New("unknown network interface")

	networkMap = map[string]string{
		"ip4:icmp": "udp4",
		"ip4:1":    "udp4",
		"ip6:icmp": "udp6",
		"ip6:58":   "udp6",
	}
)

func ListenICMP(network string, laddr string) (net.PacketConn, error) {
	// 为兼容NE，需要注释掉
	//if os.Getuid() == 0 { // root
	//	return net.ListenPacket(network, laddr)
	//} else {
	if nw, ok := networkMap[network]; ok {
		proto := syscall.IPPROTO_ICMP
		if nw == "udp6" {
			proto = syscall.IPPROTO_ICMPV6
		}

		var ifIndex = -1
		if laddr != "" {
			la := net.ParseIP(laddr)
			if ifaces, err := net.Interfaces(); err == nil {
				for _, iface := range ifaces {
					addrs, err := iface.Addrs()
					if err != nil {
						continue
					}
					for _, addr := range addrs {
						if ipnet, ok := addr.(*net.IPNet); ok {
							if ipnet.IP.Equal(la) {
								ifIndex = iface.Index
								break
							}
						}
					}
				}
				if ifIndex == -1 {
					return nil, errUnknownIface
				}
			} else {
				return nil, err
			}
		}

		isock, err := internetSocket(context.Background(), nw, nil, nil, syscall.SOCK_DGRAM, proto, "listen",
			func(ctx context.Context, network, address string, c syscall.RawConn) error {
				if ifIndex != -1 {
					if proto == syscall.IPPROTO_ICMP {
						return c.Control(func(fd uintptr) {
							err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_BOUND_IF, ifIndex)
							if err != nil {
								return
							}
						})
					} else {
						return c.Control(func(fd uintptr) {
							err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IPV6, syscall.IPV6_BOUND_IF, ifIndex)
							if err != nil {
								return
							}
						})
					}
				}
				return nil
			})
		if err != nil {
			panic(err)
		}
		return newIPConn(isock), nil
	} else {
		return nil, errUnknownNetwork
	}
	//}
}
