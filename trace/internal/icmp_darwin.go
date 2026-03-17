//go:build darwin

package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/nxtrace/NTrace-core/util"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type ICMPSpec struct {
	IPVersion    int
	ICMPMode     int
	EchoID       int
	SrcIP        net.IP
	DstIP        net.IP
	icmp         net.PacketConn
	icmp4        *ipv4.PacketConn
	icmp6        *ipv6.PacketConn
	hopLimitLock sync.Mutex
}

// ---------------------------------------------------------------------------
// icmpPacketConn 将 ICMP DGRAM socket 包装为 net.PacketConn，
// 通过 os.File + syscall.RawConn 正确集成到 Go 运行时 poller，
// 同时保持 ICMP 语义（ReadFrom 返回 *net.IPAddr）。
// 这样可完全避免 //go:linkname 依赖。
// ---------------------------------------------------------------------------

type icmpPacketConn struct {
	file *os.File
	rc   syscall.RawConn
	af   int // syscall.AF_INET or syscall.AF_INET6
}

// 编译期断言：icmpPacketConn 实现 net.PacketConn + net.Conn + syscall.Conn
var (
	_ net.PacketConn = (*icmpPacketConn)(nil)
	_ net.Conn       = (*icmpPacketConn)(nil)
	_ syscall.Conn   = (*icmpPacketConn)(nil)
)

func (c *icmpPacketConn) ReadFrom(b []byte) (int, net.Addr, error) {
	var (
		n       int
		addr    net.Addr
		readErr error
	)
	err := c.rc.Read(func(fd uintptr) bool {
		var sa syscall.Sockaddr
		n, sa, readErr = syscall.Recvfrom(int(fd), b, 0)
		if readErr == syscall.EAGAIN || readErr == syscall.EWOULDBLOCK {
			return false // 未就绪，让 poller 继续等待
		}
		if sa != nil {
			switch s := sa.(type) {
			case *syscall.SockaddrInet4:
				ip := make(net.IP, 4)
				copy(ip, s.Addr[:])
				addr = &net.IPAddr{IP: ip}
			case *syscall.SockaddrInet6:
				ip := make(net.IP, 16)
				copy(ip, s.Addr[:])
				addr = &net.IPAddr{IP: ip, Zone: zoneToName(s.ZoneId)}
			}
		}
		return true
	})
	if err != nil {
		return 0, nil, err
	}
	if readErr != nil {
		return 0, nil, readErr
	}
	// macOS DGRAM ICMP socket 返回数据包含外层 IP 头；
	// 模拟 net.IPConn.ReadFrom 行为，将其剥离以保持与解析层兼容。
	if c.af == syscall.AF_INET {
		n = stripIPv4Header(n, b)
	}
	return n, addr, nil
}

func (c *icmpPacketConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	sa, err := addrToSockaddr(addr, c.af)
	if err != nil {
		return 0, err
	}
	var writeErr error
	err = c.rc.Write(func(fd uintptr) bool {
		writeErr = syscall.Sendto(int(fd), b, 0, sa)
		if writeErr == syscall.EAGAIN || writeErr == syscall.EWOULDBLOCK {
			return false
		}
		return true
	})
	if err != nil {
		return 0, err
	}
	if writeErr != nil {
		return 0, writeErr
	}
	return len(b), nil
}

func (c *icmpPacketConn) Close() error { return c.file.Close() }

func (c *icmpPacketConn) LocalAddr() net.Addr {
	if c.af == syscall.AF_INET6 {
		return &net.IPAddr{IP: net.IPv6zero}
	}
	return &net.IPAddr{IP: net.IPv4zero}
}

func (c *icmpPacketConn) RemoteAddr() net.Addr               { return nil }
func (c *icmpPacketConn) SetDeadline(t time.Time) error      { return c.file.SetDeadline(t) }
func (c *icmpPacketConn) SetReadDeadline(t time.Time) error  { return c.file.SetReadDeadline(t) }
func (c *icmpPacketConn) SetWriteDeadline(t time.Time) error { return c.file.SetWriteDeadline(t) }

// Read 实现 net.Conn 接口（ipv4.NewPacketConn 内部需要）。
// 对于无连接 ICMP socket，Read 等价于 ReadFrom 但丢弃源地址。
func (c *icmpPacketConn) Read(b []byte) (int, error) {
	n, _, err := c.ReadFrom(b)
	return n, err
}

// Write 实现 net.Conn 接口。对于无连接 socket 不可用。
func (c *icmpPacketConn) Write(b []byte) (int, error) {
	return 0, errors.New("Write not supported on unconnected ICMP socket; use WriteTo")
}

// ReadMsgIP 实现 x/net/internal/socket.ipConn 接口，使得
// socket.NewConn 能识别此连接为 "ip" 类型，从而正确初始化 socket.Conn。
// NTrace 不实际调用此方法（读取走 ReadFrom / PacketListener），仅为接口满足。
func (c *icmpPacketConn) ReadMsgIP(b, oob []byte) (n, oobn, flags int, addr *net.IPAddr, err error) {
	var rn int
	var rAddr *net.IPAddr
	var readErr error
	err = c.rc.Read(func(fd uintptr) bool {
		var sa syscall.Sockaddr
		rn, _, readErr, sa = recvmsgRaw(int(fd), b, oob)
		if readErr == syscall.EAGAIN || readErr == syscall.EWOULDBLOCK {
			return false
		}
		if sa != nil {
			switch s := sa.(type) {
			case *syscall.SockaddrInet4:
				ip := make(net.IP, 4)
				copy(ip, s.Addr[:])
				rAddr = &net.IPAddr{IP: ip}
			case *syscall.SockaddrInet6:
				ip := make(net.IP, 16)
				copy(ip, s.Addr[:])
				rAddr = &net.IPAddr{IP: ip, Zone: zoneToName(s.ZoneId)}
			}
		}
		return true
	})
	if err != nil {
		return 0, 0, 0, nil, err
	}
	if readErr != nil {
		return 0, 0, 0, nil, readErr
	}
	return rn, 0, 0, rAddr, nil
}

// recvmsgRaw 使用 Recvfrom 实现简化版 recvmsg（不处理 OOB/control message）。
// 对于 ICMP DGRAM socket，内核不会为我们生成 IP 头 control message，
// 因此 OOB 数据始终为空。
func recvmsgRaw(fd int, b, oob []byte) (n, oobn int, err error, sa syscall.Sockaddr) {
	n, sa, err = syscall.Recvfrom(fd, b, 0)
	return n, 0, err, sa
}

// SyscallConn 让 ipv4.NewPacketConn / ipv6.NewPacketConn 能通过
// setsockopt 设置 IP_TTL / IPV6_UNICAST_HOPS 等选项。
func (c *icmpPacketConn) SyscallConn() (syscall.RawConn, error) { return c.rc, nil }

// addrToSockaddr 将 net.Addr 转换为 syscall.Sockaddr
func addrToSockaddr(addr net.Addr, af int) (syscall.Sockaddr, error) {
	var ip net.IP
	switch a := addr.(type) {
	case *net.IPAddr:
		ip = a.IP
	case *net.UDPAddr:
		ip = a.IP
	default:
		return nil, fmt.Errorf("icmpPacketConn: unsupported addr type %T", addr)
	}
	if af == syscall.AF_INET {
		sa := &syscall.SockaddrInet4{}
		copy(sa.Addr[:], ip.To4())
		return sa, nil
	}
	sa := &syscall.SockaddrInet6{}
	copy(sa.Addr[:], ip.To16())
	return sa, nil
}

// stripIPv4Header 剥离 macOS DGRAM ICMP socket 返回数据中的 IPv4 头。
// 逻辑与 Go 标准库 net.stripIPv4Header 一致（iprawsock_posix.go）。
func stripIPv4Header(n int, b []byte) int {
	if len(b) < 20 {
		return n
	}
	l := int(b[0]&0x0f) << 2
	if 20 > l || l > len(b) {
		return n
	}
	if b[0]>>4 != 4 {
		return n
	}
	copy(b, b[l:])
	return n - l
}

// zoneToName 将 IPv6 zone ID 转换为接口名
func zoneToName(idx uint32) string {
	if idx == 0 {
		return ""
	}
	iface, err := net.InterfaceByIndex(int(idx))
	if err != nil {
		return ""
	}
	return iface.Name
}

var (
	errUnknownNetwork = errors.New("unknown network type")
	errUnknownIface   = errors.New("unknown network interface")
	networkMap        = map[string]string{
		"ip4:icmp":      "udp4",
		"ip4:1":         "udp4",
		"ip6:ipv6-icmp": "udp6",
		"ip6:58":        "udp6",
	}
)

type darwinICMPSocketSpec struct {
	af    int
	proto int
}

func darwinICMPSocketSpecForNetwork(network string) (darwinICMPSocketSpec, error) {
	nw, ok := networkMap[network]
	if !ok {
		return darwinICMPSocketSpec{}, errUnknownNetwork
	}
	if nw == "udp6" {
		return darwinICMPSocketSpec{af: syscall.AF_INET6, proto: syscall.IPPROTO_ICMPV6}, nil
	}
	return darwinICMPSocketSpec{af: syscall.AF_INET, proto: syscall.IPPROTO_ICMP}, nil
}

func mustOpenDarwinICMPSocket(spec darwinICMPSocketSpec) int {
	fd, err := syscall.Socket(spec.af, syscall.SOCK_DGRAM, spec.proto)
	if err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("ListenPacket: socket: %w", err))
		}
		log.Fatalf("ListenPacket: socket: %v", err)
	}
	return fd
}

func interfaceHasIP(iface net.Interface, target net.IP) bool {
	addrs, err := iface.Addrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if ok && ipnet.IP.Equal(target) {
			return true
		}
	}
	return false
}

func interfaceIndexByIP(ip net.IP) (int, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return -1, err
	}
	for _, iface := range ifaces {
		if interfaceHasIP(iface, ip) {
			return iface.Index, nil
		}
	}
	return -1, errUnknownIface
}

func setDarwinBoundInterface(fd, proto, ifIndex int) error {
	if proto == syscall.IPPROTO_ICMP {
		if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_BOUND_IF, ifIndex); err != nil {
			return fmt.Errorf("setsockopt IP_BOUND_IF: %w", err)
		}
		return nil
	}
	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_IPV6, syscall.IPV6_BOUND_IF, ifIndex); err != nil {
		return fmt.Errorf("setsockopt IPV6_BOUND_IF: %w", err)
	}
	return nil
}

func bindDarwinICMPInterface(fd, proto int, laddr string) error {
	if laddr == "" {
		return nil
	}
	ifIndex, err := interfaceIndexByIP(net.ParseIP(laddr))
	if err != nil {
		return err
	}
	return setDarwinBoundInterface(fd, proto, ifIndex)
}

func darwinICMPBindSockaddr(af int, laddr string) syscall.Sockaddr {
	if af == syscall.AF_INET {
		bindAddr := &syscall.SockaddrInet4{}
		if ip4 := net.ParseIP(laddr).To4(); ip4 != nil {
			copy(bindAddr.Addr[:], ip4)
		}
		return bindAddr
	}

	bindAddr := &syscall.SockaddrInet6{}
	if ip6 := net.ParseIP(laddr).To16(); ip6 != nil {
		copy(bindAddr.Addr[:], ip6)
	}
	return bindAddr
}

func bindDarwinICMPSocket(fd, af int, laddr string) error {
	if err := syscall.Bind(fd, darwinICMPBindSockaddr(af, laddr)); err != nil {
		return fmt.Errorf("bind: %w", err)
	}
	return nil
}

func finalizeDarwinICMPSocket(fd, af int) (net.PacketConn, error) {
	if err := syscall.SetNonblock(fd, true); err != nil {
		_ = syscall.Close(fd)
		return nil, fmt.Errorf("setnonblock: %w", err)
	}

	f := os.NewFile(uintptr(fd), "icmp")
	if f == nil {
		_ = syscall.Close(fd)
		return nil, fmt.Errorf("os.NewFile returned nil")
	}

	rc, err := f.SyscallConn()
	if err != nil {
		_ = f.Close()
		if util.EnvDevMode {
			panic(fmt.Errorf("ListenPacket: SyscallConn: %w", err))
		}
		log.Fatalf("ListenPacket: SyscallConn: %v", err)
	}

	return &icmpPacketConn{file: f, rc: rc, af: af}, nil
}

func ListenPacket(network string, laddr string) (net.PacketConn, error) {
	spec, err := darwinICMPSocketSpecForNetwork(network)
	if err != nil {
		return nil, err
	}

	fd := mustOpenDarwinICMPSocket(spec)
	if err := bindDarwinICMPInterface(fd, spec.proto, laddr); err != nil {
		_ = syscall.Close(fd)
		return nil, err
	}
	if err := bindDarwinICMPSocket(fd, spec.af, laddr); err != nil {
		_ = syscall.Close(fd)
		return nil, err
	}

	return finalizeDarwinICMPSocket(fd, spec.af)
}

func (s *ICMPSpec) Close() {
	_ = s.icmp.Close()
}

func (s *ICMPSpec) ListenICMP(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, seq int)) {
	s.listenICMPSock(ctx, ready, onICMP)
}

func (s *ICMPSpec) SendICMP(ctx context.Context, ipHdr gopacket.NetworkLayer, icmpHdr, icmpEcho gopacket.SerializableLayer, payload []byte) (time.Time, error) {
	select {
	case <-ctx.Done():
		return time.Time{}, context.Canceled
	default:
	}

	if s.IPVersion == 4 {
		ip4, ok := ipHdr.(*layers.IPv4)
		if !ok || ip4 == nil {
			return time.Time{}, errors.New("SendICMP: expect *layers.IPv4 when s.IPVersion==4")
		}
		ttl := int(ip4.TTL)

		buf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{
			ComputeChecksums: true,
			FixLengths:       true,
		}

		// 序列化 ICMP 头与 payload 到缓冲区
		if err := gopacket.SerializeLayers(buf, opts, icmpHdr, gopacket.Payload(payload)); err != nil {
			return time.Time{}, err
		}

		// 串行设置 TTL + 发送，放在同一把锁里保证并发安全
		s.hopLimitLock.Lock()
		defer s.hopLimitLock.Unlock()

		if err := s.icmp4.SetTOS(int(ip4.TOS)); err != nil {
			return time.Time{}, err
		}
		if err := s.icmp4.SetTTL(ttl); err != nil {
			return time.Time{}, err
		}

		start := time.Now()

		if _, err := s.icmp.WriteTo(buf.Bytes(), &net.IPAddr{IP: s.DstIP}); err != nil {
			return time.Time{}, err
		}
		return start, nil
	}

	ip6, ok := ipHdr.(*layers.IPv6)
	if !ok || ip6 == nil {
		return time.Time{}, errors.New("SendICMP: expect *layers.IPv6 when s.IPVersion==6")
	}
	ttl := int(ip6.HopLimit)

	ic6, ok := icmpHdr.(*layers.ICMPv6)
	if !ok || ic6 == nil {
		return time.Time{}, errors.New("SendICMP: expect *layers.ICMPv6 when s.IPVersion==6")
	}

	if err := ic6.SetNetworkLayerForChecksum(ipHdr); err != nil {
		return time.Time{}, fmt.Errorf("SetNetworkLayerForChecksum: %w", err)
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	// 序列化 ICMP 头与 payload 到缓冲区
	if err := gopacket.SerializeLayers(buf, opts, icmpHdr, icmpEcho, gopacket.Payload(payload)); err != nil {
		return time.Time{}, err
	}

	// 串行设置 HopLimit + 发送，放在同一把锁里保证并发安全
	s.hopLimitLock.Lock()
	defer s.hopLimitLock.Unlock()

	if err := s.icmp6.SetTrafficClass(int(ip6.TrafficClass)); err != nil {
		return time.Time{}, err
	}
	if err := s.icmp6.SetHopLimit(ttl); err != nil {
		return time.Time{}, err
	}

	start := time.Now()

	if _, err := s.icmp.WriteTo(buf.Bytes(), &net.IPAddr{IP: s.DstIP}); err != nil {
		return time.Time{}, err
	}
	return start, nil
}
