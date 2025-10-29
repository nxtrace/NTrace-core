package util

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"

	"github.com/nxtrace/NTrace-core/config"
)

var SrcDev string
var SrcPort int
var DstIP string
var PowProviderParam = ""
var rDNSCache sync.Map
var UserAgent = fmt.Sprintf("NextTrace %s/%s/%s", config.Version, runtime.GOOS, runtime.GOARCH)
var cachedLocalIP net.IP
var cachedLocalPort int
var localIPOnce sync.Once
var cachedLocalIPv6 net.IP
var cachedLocalPort6 int
var localIPv6Once sync.Once

func IsIPv6(ip net.IP) bool {
	return ip != nil && ip.To4() == nil && ip.To16() != nil
}

// AddrIP 从常见的 net.Addr 中提取 IP：支持 *net.IPAddr / *net.TCPAddr / *net.UDPAddr
// 若无法提取，返回 nil
func AddrIP(a net.Addr) net.IP {
	switch addr := a.(type) {
	case *net.IPAddr:
		return addr.IP
	case *net.TCPAddr:
		return addr.IP
	case *net.UDPAddr:
		return addr.IP
	default:
		return nil
	}
}

func RandomPortEnabled() bool {
	return EnvRandomPort || SrcPort == -1
}

func LookupAddr(addr string) ([]string, error) {
	// 如果在缓存中找到，直接返回
	if hostname, ok := rDNSCache.Load(addr); ok {
		//fmt.Println("hit rDNSCache for", addr, hostname)
		return []string{hostname.(string)}, nil
	}

	// 如果缓存中未找到，进行 DNS 查询
	names, err := net.LookupAddr(addr)
	if err != nil {
		return nil, err
	}

	// 将查询结果存入缓存
	if len(names) > 0 {
		rDNSCache.Store(addr, names[0])
	}
	return names, nil
}

// getLocalIPPort（仅用于 IPv4）：
// (1) 若 srcIP 非空，则以其为绑定源 IP；否则先通过 DialUDP 到 dstIP 获取实际出站源 IP
// (2) 根据 proto：
//
//	"icmp"     ：直接返回 bindIP，bindPort=0（表示“无端口”）；
//	"tcp"/"udp"：使用 Listen* 以 Port=0 做一次本地绑定测试，让内核分配可用端口，并在记录后立即关闭
//
// (3) 立即关闭监听并返回 (bindIP, bindPort)，若出错则返回 (nil, -1)
func getLocalIPPort(dstIP net.IP, srcIP net.IP, proto string) (net.IP, int) {
	if dstIP == nil || dstIP.To4() == nil {
		return nil, -1
	}

	// (1) 选定 bindIP：优先使用显式 srcIP，否则通过 UDP 伪 connect 探测
	var bindIP net.IP
	if srcIP != nil && srcIP.To4() != nil {
		bindIP = srcIP
	} else {
		serverAddr := &net.UDPAddr{IP: dstIP, Port: 12345}
		con, err := net.DialUDP("udp4", nil, serverAddr)
		if err != nil {
			return nil, -1
		}
		la, _ := con.LocalAddr().(*net.UDPAddr)
		_ = con.Close()
		if la == nil || la.IP == nil || la.IP.To4() == nil {
			return nil, -1
		}
		bindIP = la.IP
	}

	// (2) 按需求测试端口可用性（仅本地 bind，不做网络握手）
	switch proto {
	case "icmp":
		return bindIP, 0
	case "tcp":
		ln, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: bindIP, Port: 0})
		if err != nil {
			return nil, -1
		}
		bindPort := ln.Addr().(*net.TCPAddr).Port
		_ = ln.Close()
		return bindIP, bindPort
	case "udp":
		pc, err := net.ListenUDP("udp4", &net.UDPAddr{IP: bindIP, Port: 0})
		if err != nil {
			return nil, -1
		}
		bindPort := pc.LocalAddr().(*net.UDPAddr).Port
		_ = pc.Close()
		return bindIP, bindPort
	}
	return nil, -1
}

// getLocalIPPortv6（仅用于 IPv6）：
// (1) 若 srcIP 非空，则以其为绑定源 IP；否则先通过 DialUDP 到 dstIP 获取实际出站源 IP
// (2) 根据 proto：
//
//	"icmp6"      ：直接返回 bindIP，bindPort=0（表示“无端口”）；
//	"tcp6"/"udp6"：使用 Listen* 以 Port=0 做一次本地绑定测试，让内核分配可用端口，并在记录后立即关闭
//
// (3) 立即关闭监听并返回 (bindIP, bindPort)，若出错则返回 (nil, -1)
func getLocalIPPortv6(dstIP net.IP, srcIP net.IP, proto string) (net.IP, int) {
	if !IsIPv6(dstIP) {
		return nil, -1
	}

	// (1) 选定 bindIP：优先使用显式 srcIP，否则通过 UDP 伪 connect 探测
	var bindIP net.IP
	if srcIP != nil && IsIPv6(srcIP) {
		bindIP = srcIP
	} else {
		serverAddr := &net.UDPAddr{IP: dstIP, Port: 12345}
		con, err := net.DialUDP("udp6", nil, serverAddr)
		if err != nil {
			return nil, -1
		}
		la, _ := con.LocalAddr().(*net.UDPAddr)
		_ = con.Close()
		if la == nil || !IsIPv6(la.IP) {
			return nil, -1
		}
		bindIP = la.IP
	}

	// (2) 按需求测试端口可用性（仅本地 bind，不做网络握手）
	switch proto {
	case "icmp6":
		return bindIP, 0
	case "tcp6":
		ln, err := net.ListenTCP("tcp6", &net.TCPAddr{IP: bindIP, Port: 0})
		if err != nil {
			return nil, -1
		}
		bindPort := ln.Addr().(*net.TCPAddr).Port
		_ = ln.Close()
		return bindIP, bindPort
	case "udp6":
		pc, err := net.ListenUDP("udp6", &net.UDPAddr{IP: bindIP, Port: 0})
		if err != nil {
			return nil, -1
		}
		bindPort := pc.LocalAddr().(*net.UDPAddr).Port
		_ = pc.Close()
		return bindIP, bindPort
	}
	return nil, -1
}

// LocalIPPort 根据目标 IPv4（以及可选的源 IPv4 与协议）返回本地 IP 与一个可用端口
func LocalIPPort(dstIP net.IP, srcIP net.IP, proto string) (net.IP, int) {
	// 若开启随机端口模式，每次直接计算并返回
	if RandomPortEnabled() {
		return getLocalIPPort(dstIP, srcIP, proto)
	}

	// 否则仅计算一次并缓存
	localIPOnce.Do(func() {
		cachedLocalIP, cachedLocalPort = getLocalIPPort(dstIP, srcIP, proto)
	})

	if cachedLocalIP != nil {
		return cachedLocalIP, cachedLocalPort
	}
	return nil, -1
}

// LocalIPPortv6 根据目标 IPv6（以及可选的源 IPv6 与协议）返回本地 IP 与一个可用端口
func LocalIPPortv6(dstIP net.IP, srcIP net.IP, proto string) (net.IP, int) {
	// 若开启随机端口模式，每次直接计算并返回
	if RandomPortEnabled() {
		return getLocalIPPortv6(dstIP, srcIP, proto)
	}

	// 否则仅计算一次并缓存
	localIPv6Once.Do(func() {
		cachedLocalIPv6, cachedLocalPort6 = getLocalIPPortv6(dstIP, srcIP, proto)
	})

	if cachedLocalIPv6 != nil {
		return cachedLocalIPv6, cachedLocalPort6
	}
	return nil, -1
}

func DomainLookUp(host string, ipVersion string, dotServer string, disableOutput bool) (net.IP, error) {
	// ipVersion: 4, 6, all
	var (
		r   *net.Resolver
		ips []net.IP
	)

	switch dotServer {
	case "dnssb":
		r = DNSSB()
	case "aliyun":
		r = Aliyun()
	case "dnspod":
		r = Dnspod()
	case "google":
		r = Google()
	case "cloudflare":
		r = Cloudflare()
	default:
		r = newUDPResolver()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ipsStr, err := r.LookupHost(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("DNS lookup failed: %w", err)
	}
	for _, v := range ipsStr {
		if parsed := net.ParseIP(v); parsed != nil {
			ips = append(ips, parsed)
		}
	}

	// Filter by IPv4/IPv6
	if ipVersion != "all" {
		var filteredIPs []net.IP
		for _, ip := range ips {
			if ip == nil {
				continue
			}
			if ipVersion == "4" && ip.To4() != nil {
				filteredIPs = []net.IP{ip}
				break
			} else if ipVersion == "6" && ip.To4() == nil {
				filteredIPs = []net.IP{ip}
				break
			}
		}
		ips = filteredIPs
	}

	if len(ips) == 0 {
		var familyLabel string
		switch ipVersion {
		case "4":
			familyLabel = "IPv4"
		case "6":
			familyLabel = "IPv6"
		case "all", "":
			familyLabel = "IPv4/IPv6"
		default:
			familyLabel = ipVersion
		}
		return nil, fmt.Errorf("no %s DNS records found for %s", familyLabel, host)
	}

	if (len(ips) == 1) || (disableOutput) {
		return ips[0], nil
	}

	fmt.Println("Please Choose the IP You Want To TraceRoute")
	for i, ip := range ips {
		_, _ = fmt.Fprintf(color.Output, "%s %s\n",
			color.New(color.FgHiYellow, color.Bold).Sprintf("%d.", i),
			color.New(color.FgWhite, color.Bold).Sprintf("%s", ip),
		)
	}
	var index int
	fmt.Printf("Your Option: ")
	_, err = fmt.Scanln(&index)
	if err != nil {
		index = 0
	}
	if index >= len(ips) || index < 0 {
		fmt.Println("Your Option is invalid")
		return nil, fmt.Errorf("invalid selection: %d", index)
	}
	return ips[index], nil
}

func GetHostAndPort() (host string, port string) {
	// 解析域名
	hostArr := strings.Split(EnvHostPort, ":")
	// 判断是否有指定端口
	if len(hostArr) > 1 {
		// 判断是否为 IPv6
		if strings.HasPrefix(EnvHostPort, "[") {
			tmp := strings.Split(EnvHostPort, "]")
			host = tmp[0]
			host = host[1:]
			if port = tmp[1]; port != "" {
				port = port[1:]
			}
		} else {
			host, port = hostArr[0], hostArr[1]
		}
	} else {
		host = EnvHostPort
	}
	if port == "" {
		// 默认端口
		port = "443"
	}
	return
}

func GetProxy() *url.URL {
	if EnvProxyURL == "" {
		return nil
	}
	proxyURL, err := url.Parse(EnvProxyURL)
	if err != nil {
		log.Println("Failed to parse proxy URL:", err)
		return nil
	}
	return proxyURL
}

func GetPowProvider() string {
	var powProvider string
	if PowProviderParam == "" {
		powProvider = EnvPowProvider
	} else {
		powProvider = PowProviderParam
	}
	if powProvider == "sakura" {
		return "pow.nexttrace.owo.13a.com"
	}
	return ""
}

func StringInSlice(val string, list []string) bool {
	for _, v := range list {
		if v == val {
			return true
		}
	}
	return false
}

func HideIPPart(ip string) string {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return ""
	}

	if parsedIP.To4() != nil {
		// IPv4: 隐藏后16位
		return strings.Join(strings.Split(ip, ".")[:2], ".") + ".0.0/16"
	}
	// IPv6: 隐藏后96位
	return parsedIP.Mask(net.CIDRMask(32, 128)).String() + "/32"
}

// fold16 对 32 位累加和做 16 位一补折叠（包含环回进位）
func fold16(sum uint32) uint16 {
	// 将高 16 位进位折叠回低 16 位，直至无进位
	for (sum >> 16) != 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}

	return uint16(sum & 0xFFFF)
}

// addBytes 按大端序把字节流每 16 位累加到 sum；若长度为奇数，最后 1 字节作为高位、低位补 0
func addBytes(sum uint32, data []byte) uint32 {
	// 按大端序每次取两个字节，将合成后的 16 位无符号数累加到 sum
	for i := 0; i+1 < len(data); i += 2 {
		sum += uint32(data[i])<<8 | uint32(data[i+1])
	}

	// 奇数字节则末尾补 0
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}

	return sum
}

// UDPBaseSum 在“UDP.Checksum 视为 0、payload[0:2]=0x0000”的前提下，计算 16 位一补和 S0
func UDPBaseSum(srcIP, dstIP net.IP, srcPort, dstPort, udpLen int, payload []byte) uint16 {
	sum := uint32(0)

	if src4 := srcIP.To4(); src4 != nil {
		dst4 := dstIP.To4()
		sum = addBytes(sum, src4)
		sum = addBytes(sum, dst4)
		sum += uint32(0x0011)
		sum += uint32(udpLen & 0xFFFF)
	} else {
		src6 := srcIP.To16()
		dst6 := dstIP.To16()
		sum = addBytes(sum, src6)
		sum = addBytes(sum, dst6)

		uLen := uint32(udpLen)
		sum += (uLen >> 16) & 0xFFFF
		sum += uLen & 0xFFFF

		sum += uint32(0x0011)
	}
	sum += uint32(srcPort & 0xFFFF)
	sum += uint32(dstPort & 0xFFFF)
	sum += uint32(udpLen & 0xFFFF)

	sum = addBytes(sum, payload)

	return fold16(sum)
}

// FudgeWordForSeq 给定 S0 与目标 checksum（如 seq），返回应写入 payload[0:2] 的补偿值（16 位）
// 原理：最终一补和 targetSum = ^targetChecksum；令补偿位 X，则 X = targetSum ⊕ (~S0)
func FudgeWordForSeq(S0, targetChecksum uint16) uint16 {
	targetSum := ^targetChecksum         // 目标一补和
	x := uint32(targetSum) + uint32(^S0) // X = targetSum ⊕ (~S0)
	return fold16(x)
}

// MakePayloadWithTargetChecksum 修改 payload，使最终 UDP.Checksum == targetChecksum
// 要求：payload 长度 >= 2（前 2 字节作为补偿位写入）
func MakePayloadWithTargetChecksum(payload []byte, srcIP, dstIP net.IP, srcPort, dstPort int, targetChecksum uint16) error {
	if len(payload) < 2 {
		return errors.New("payload too short, need >= 2 bytes for fudge")
	}

	// v4/v6 一致性校验
	if (srcIP.To4() == nil) != (dstIP.To4() == nil) {
		return errors.New("src/dst IP version mismatch (v4/v6)")
	}

	// 补偿位清零，再按“校验和字段=0”的前提计算 S0
	payload[0], payload[1] = 0, 0
	udpLen := 8 + len(payload)
	S0 := UDPBaseSum(srcIP, dstIP, srcPort, dstPort, udpLen, payload)
	fudge := FudgeWordForSeq(S0, targetChecksum)

	// 回写补偿位（网络序）
	payload[0] = byte(fudge >> 8)
	payload[1] = byte(fudge)
	return nil
}
