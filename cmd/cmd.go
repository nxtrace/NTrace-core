package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"github.com/akamensky/argparse"
	"github.com/fatih/color"
	"github.com/syndtr/gocapability/capability"

	"github.com/nxtrace/NTrace-core/assets/windivert"
	"github.com/nxtrace/NTrace-core/config"
	fastTrace "github.com/nxtrace/NTrace-core/fast_trace"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/reporter"
	"github.com/nxtrace/NTrace-core/server"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/tracelog"
	"github.com/nxtrace/NTrace-core/tracemap"
	"github.com/nxtrace/NTrace-core/util"
	"github.com/nxtrace/NTrace-core/wshandle"
)

type listenInfo struct {
	Binding string
	Access  string
}

const (
	defaultPacketIntervalMs        = 50
	defaultTracerouteTTLIntervalMs = 300
)

func buildListenInfo(addr string) listenInfo {
	trimmed := strings.TrimSpace(addr)
	effective := trimmed
	if trimmed != "" && isDigitsOnly(trimmed) {
		effective = ":" + trimmed
	}

	if effective == "" {
		effective = ":1080"
	}

	host, port, err := net.SplitHostPort(effective)
	if err != nil {
		if strings.HasPrefix(effective, ":") {
			host = ""
			port = strings.TrimPrefix(effective, ":")
		} else {
			return listenInfo{
				Binding: effective,
			}
		}
	}

	if port == "" {
		port = "1080"
	}

	rawHost := host
	if rawHost == "" {
		rawHost = "0.0.0.0"
	}

	bindingHost := rawHost
	if strings.Contains(bindingHost, ":") && !strings.HasPrefix(bindingHost, "[") {
		bindingHost = "[" + bindingHost + "]"
	}

	info := listenInfo{
		Binding: fmt.Sprintf("http://%s:%s", bindingHost, port),
	}

	wildcard := host == "" || host == "0.0.0.0" || host == "::"
	var accessHost string
	if wildcard {
		accessHost = guessLocalIPv4()
	} else {
		accessHost = host
	}

	if accessHost != "" {
		if strings.Contains(accessHost, ":") && !strings.HasPrefix(accessHost, "[") {
			accessHost = "[" + accessHost + "]"
		}
		info.Access = fmt.Sprintf("http://%s:%s", accessHost, port)
	}

	return info
}

func isDigitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func guessLocalIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, address := range addrs {
			if ipNet, ok := address.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				if ip4 := ipNet.IP.To4(); ip4 != nil {
					return ip4.String()
				}
			}
		}
	}
	return "127.0.0.1"
}

func defaultLocalListenAddr() string {
	if hasIPv4Loopback() {
		return "127.0.0.1:1080"
	}
	if hasIPv6Loopback() {
		return "[::1]:1080"
	}
	return "127.0.0.1:1080"
}

func hasIPv4Loopback() bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, address := range addrs {
		if ipNet, ok := address.(*net.IPNet); ok && ipNet.IP.IsLoopback() {
			if ip4 := ipNet.IP.To4(); ip4 != nil {
				return true
			}
		}
	}
	return false
}

func hasIPv6Loopback() bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, address := range addrs {
		if ipNet, ok := address.(*net.IPNet); ok && ipNet.IP.IsLoopback() {
			if ip := ipNet.IP; ip.To4() == nil && len(ip) == net.IPv6len {
				return true
			}
		}
	}
	return false
}

func Execute() {
	parser := argparse.NewParser("nexttrace", "An open source visual route tracking CLI tool")
	// Create string flag
	init := parser.Flag("", "init", &argparse.Options{Help: "Windows ONLY: Extract WinDivert runtime to current directory"})
	ipv4Only := parser.Flag("4", "ipv4", &argparse.Options{Help: "Use IPv4 only"})
	ipv6Only := parser.Flag("6", "ipv6", &argparse.Options{Help: "Use IPv6 only"})
	tcp := parser.Flag("T", "tcp", &argparse.Options{Help: "Use TCP SYN for tracerouting (default dest-port is 80)"})
	udp := parser.Flag("U", "udp", &argparse.Options{Help: "Use UDP SYN for tracerouting (default dest-port is 33494)"})
	fast_trace := parser.Flag("F", "fast-trace", &argparse.Options{Help: "One-Key Fast Trace to China ISPs"})
	port := parser.Int("p", "port", &argparse.Options{Help: "Set the destination port to use. With default of 80 for \"tcp\", 33494 for \"udp\""})
	icmpMode := parser.Int("", "icmp-mode", &argparse.Options{Help: "Windows ONLY: Choose the method to listen for ICMP packets (1=Socket, 2=WinDivert; 0=Auto)"})
	numMeasurements := parser.Int("q", "queries", &argparse.Options{Default: 3, Help: "Set the number of latency samples to display for each hop"})
	maxAttempts := parser.Int("", "max-attempts", &argparse.Options{Help: "Set the maximum number of probe packets per hop (instead of a fixed auto value)"})
	parallelRequests := parser.Int("", "parallel-requests", &argparse.Options{Default: 18, Help: "Set ParallelRequests number. It should be 1 when there is a multi-routing"})
	maxHops := parser.Int("m", "max-hops", &argparse.Options{Default: 30, Help: "Set the max number of hops (max TTL to be reached)"})
	dataOrigin := parser.Selector("d", "data-provider", []string{"IP.SB", "ip.sb", "IPInfo", "ipinfo", "IPInsight", "ipinsight", "IPAPI.com", "ip-api.com", "IPInfoLocal", "ipinfolocal", "chunzhen", "LeoMoeAPI", "leomoeapi", "ipdb.one", "disable-geoip"}, &argparse.Options{Default: "LeoMoeAPI",
		Help: "Choose IP Geograph Data Provider [IP.SB, IPInfo, IPInsight, IP-API.com, IPInfoLocal, CHUNZHEN, disable-geoip]"})
	powProvider := parser.Selector("", "pow-provider", []string{"api.nxtrace.org", "sakura"}, &argparse.Options{Default: "api.nxtrace.org",
		Help: "Choose PoW Provider [api.nxtrace.org, sakura] For China mainland users, please use sakura"})
	norDNS := parser.Flag("n", "no-rdns", &argparse.Options{Help: "Do not resolve IP addresses to their domain names"})
	alwaysrDNS := parser.Flag("a", "always-rdns", &argparse.Options{Help: "Always resolve IP addresses to their domain names"})
	routePath := parser.Flag("P", "route-path", &argparse.Options{Help: "Print traceroute hop path by ASN and location"})
	dn42 := parser.Flag("", "dn42", &argparse.Options{Help: "DN42 Mode"})
	output := parser.Flag("o", "output", &argparse.Options{Help: "Write trace result to file (RealTimePrinter ONLY)"})
	tablePrint := parser.Flag("", "table", &argparse.Options{Help: "Output trace results as a final summary table (traceroute report mode)"})
	rawPrint := parser.Flag("", "raw", &argparse.Options{Help: "Machine-friendly output. With MTR (--mtr/-r/-w), enables streaming raw event mode"})
	jsonPrint := parser.Flag("j", "json", &argparse.Options{Help: "Output trace results as JSON"})
	classicPrint := parser.Flag("c", "classic", &argparse.Options{Help: "Classic Output trace results like BestTrace"})
	beginHop := parser.Int("f", "first", &argparse.Options{Default: 1, Help: "Start from the first_ttl hop (instead of 1)"})
	disableMaptrace := parser.Flag("M", "map", &argparse.Options{Help: "Disable Print Trace Map"})
	disableMPLS := parser.Flag("e", "disable-mpls", &argparse.Options{Help: "Disable MPLS"})
	ver := parser.Flag("V", "version", &argparse.Options{Help: "Print version info and exit"})
	srcAddr := parser.String("s", "source", &argparse.Options{Help: "Use source address src_addr for outgoing packets"})
	srcPort := parser.Int("", "source-port", &argparse.Options{Help: "Use source port src_port for outgoing packets"})
	srcDev := parser.String("D", "dev", &argparse.Options{Help: "Use the following Network Devices as the source address in outgoing packets"})
	deployListen := parser.String("", "listen", &argparse.Options{Help: "Set listen address for web console (e.g. 127.0.0.1:30080)"})
	deploy := parser.Flag("", "deploy", &argparse.Options{Help: "Start the Gin powered web console"})
	//router := parser.Flag("R", "route", &argparse.Options{Help: "Show Routing Table [Provided By BGP.Tools]"})
	packetInterval := parser.Int("z", "send-time", &argparse.Options{Default: defaultPacketIntervalMs, Help: "Set how many [milliseconds] between sending each packet. Default: 50ms"})
	ttlInterval := parser.Int("i", "ttl-time", &argparse.Options{Default: defaultTracerouteTTLIntervalMs, Help: "Interval [ms] between TTL groups in normal traceroute (default: 300ms). In MTR mode (--mtr/-r/-w, including --raw), sets per-hop probe interval: how long between successive probes to the same hop (default: 1000ms when omitted). -z/--send-time is ignored in MTR mode"})
	timeout := parser.Int("", "timeout", &argparse.Options{Default: 1000, Help: "The number of [milliseconds] to keep probe sockets open before giving up on the connection"})
	packetSize := parser.Int("", "psize", &argparse.Options{Default: 52, Help: "Set the payload size"})
	str := parser.StringPositional(&argparse.Options{Help: "IP Address or domain name"})
	dot := parser.Selector("", "dot-server", []string{"dnssb", "aliyun", "dnspod", "google", "cloudflare"}, &argparse.Options{
		Help: "Use DoT Server for DNS Parse [dnssb, aliyun, dnspod, google, cloudflare]"})
	lang := parser.Selector("g", "language", []string{"en", "cn"}, &argparse.Options{Default: "cn",
		Help: "Choose the language for displaying [en, cn]"})
	file := parser.String("", "file", &argparse.Options{Help: "Read IP Address or domain name from file"})
	noColor := parser.Flag("C", "no-color", &argparse.Options{Help: "Disable Colorful Output"})
	from := parser.String("", "from", &argparse.Options{Help: "Run traceroute via Globalping (https://globalping.io/network) from a specified location. The location field accepts continents, countries, regions, cities, ASNs, ISPs, or cloud regions."})
	mtrMode := parser.Flag("t", "mtr", &argparse.Options{Help: "Enable MTR (My Traceroute) continuous probing mode"})
	reportMode := parser.Flag("r", "report", &argparse.Options{Help: "MTR report mode (non-interactive, implies --mtr); can trigger MTR without --mtr"})
	wideMode := parser.Flag("w", "wide", &argparse.Options{Help: "MTR wide report mode (implies --mtr --report); alone equals --mtr --report --wide"})
	showIPs := parser.Flag("", "show-ips", &argparse.Options{Help: "MTR only: display both PTR hostnames and numeric IPs (PTR first, IP in parentheses)"})
	ipInfoMode := parser.Int("y", "ipinfo", &argparse.Options{Default: 0, Help: "Set initial MTR TUI host info mode (0-4). TUI only; ignored in --report/--raw. 0:IP/PTR 1:ASN 2:City 3:Owner 4:Full"})

	err := parser.Parse(os.Args)
	if err != nil {
		// In case of error print error and print usage
		// This can also be done by passing -h or --help flags
		fmt.Print(parser.Usage(err))
		return
	}

	// ── 统一 MTR 有效开关 ──
	effectiveMTR := *mtrMode || *reportMode || *wideMode
	effectiveReport := *reportMode || *wideMode
	effectiveWide := *wideMode
	effectiveMTRRaw := effectiveMTR && *rawPrint

	// MTR 冲突检查必须在所有可能 early-return 的分支之前执行
	if effectiveMTR {
		conflictFlags := map[string]bool{
			"table":     *tablePrint,
			"classic":   *classicPrint,
			"json":      *jsonPrint,
			"output":    *output,
			"routePath": *routePath,
			"from":      *from != "",
			"fastTrace": *fast_trace,
			"file":      *file != "",
			"deploy":    *deploy,
		}
		if conflict, ok := checkMTRConflicts(conflictFlags); !ok {
			fmt.Printf("--mtr 不能与 %s 同时使用\n", conflict)
			os.Exit(1)
		}
	}

	// 判定 -q / -i 是否显式传入（用于 MTR 下参数迁移）
	queriesExplicit := false
	ttlTimeExplicit := false
	for _, a := range parser.GetArgs() {
		if !a.GetParsed() {
			continue
		}
		switch a.GetLname() {
		case "queries":
			queriesExplicit = true
		case "ttl-time":
			ttlTimeExplicit = true
		}
	}

	if *noColor {
		color.NoColor = true
	} else {
		color.NoColor = false
	}

	if !*jsonPrint && !effectiveMTR {
		printer.Version()
	}

	if *ver {
		printer.CopyRight()
		os.Exit(0)
	}

	if *deploy {
		capabilitiesCheck()
		// 优先使用 CLI 参数，其次使用环境变量
		listenAddr := strings.TrimSpace(*deployListen)
		envAddr := strings.TrimSpace(util.EnvDeployAddr)
		userProvided := listenAddr != "" || envAddr != ""
		if listenAddr == "" {
			listenAddr = envAddr
		}
		if listenAddr == "" {
			listenAddr = defaultLocalListenAddr()
		}

		info := buildListenInfo(listenAddr)
		// 判断是否同时未通过 CLI 和环境变量指定地址
		if !userProvided {
			fmt.Printf("启动 NextTrace Web 控制台，监听地址: %s\n", info.Binding)
			fmt.Println("远程访问请显式设置 --listen（例如 --listen 0.0.0.0:1080）。")
			if info.Access != "" && info.Access != info.Binding {
				fmt.Printf("如需远程访问，请尝试: %s\n", info.Access)
			}
		} else {
			fmt.Printf("启动 NextTrace Web 控制台，监听地址: %s\n", info.Binding)
			if info.Access != "" && info.Access != info.Binding {
				fmt.Printf("如需远程访问，请尝试: %s\n", info.Access)
			}
		}
		fmt.Println("注意：Web 控制台的安全性有限，请在确保安全的前提下使用，如有必要请使用ACL等方式加强安全性")
		if err := server.Run(listenAddr); err != nil {
			if util.EnvDevMode {
				panic(err)
			}
			log.Fatal(err)
		}
		return
	}

	OSType := 3
	switch runtime.GOOS {
	case "darwin":
		OSType = 1
	case "windows":
		OSType = 2
	}

	if *init && OSType == 2 {
		if err := windivert.PrepareWinDivertRuntime(); err != nil {
			if util.EnvDevMode {
				panic(err)
			}
			log.Fatal(err)
		}
		fmt.Println("WinDivert runtime is ready.")
		return
	}

	if *port == 0 {
		if *udp {
			*port = 33494
		} else {
			*port = 80
		}
	}

	if !*tcp {
		if *numMeasurements > 255 {
			fmt.Println("Query 最大值为 255，已自动调整为 255")
			*numMeasurements = 255
		}

		if *maxAttempts > 255 {
			fmt.Println("MaxAttempt 最大值为 255，已自动调整为 255")
			*maxAttempts = 255
		}
	}

	domain := *str

	// 将 --dot-server 配置注入 Geo DNS 解析策略层，
	// 使 GeoIP API 域名解析（含 LeoMoe FastIP）也走 DoT。
	// 必须在 fast-trace / wshandle.New() / GetFastIP 之前执行。
	if *dot != "" {
		util.SetGeoDNSResolver(*dot)
	}

	var m trace.Method
	switch {
	case *tcp:
		m = trace.TCPTrace
	case *udp:
		m = trace.UDPTrace
	default:
		m = trace.ICMPTrace
	}

	if *from == "" && (*fast_trace || *file != "") {
		var paramsFastTrace = fastTrace.ParamsFastTrace{
			OSType:         OSType,
			ICMPMode:       *icmpMode,
			SrcDev:         *srcDev,
			SrcAddr:        *srcAddr,
			DstPort:        *port,
			BeginHop:       *beginHop,
			MaxHops:        *maxHops,
			MaxAttempts:    *maxAttempts,
			RDNS:           !*norDNS,
			AlwaysWaitRDNS: *alwaysrDNS,
			Lang:           *lang,
			PktSize:        *packetSize,
			Timeout:        time.Duration(*timeout) * time.Millisecond,
			File:           *file,
			Dot:            *dot,
		}

		fastTrace.FastTest(m, *output, paramsFastTrace)
		if *output {
			fmt.Println("您的追踪日志已经存放在 /tmp/trace.log 中")
		}

		os.Exit(0)
	}

	// DOMAIN处理开始
	if domain == "" {
		fmt.Print(parser.Usage(err))
		return
	}

	if strings.Contains(domain, "/") {
		domain = "n" + domain
		parts := strings.Split(domain, "/")
		if len(parts) < 3 {
			fmt.Println("Invalid input")
			return
		}
		domain = parts[2]
	}

	if strings.Contains(domain, "]") {
		domain = strings.Split(strings.Split(domain, "]")[0], "[")[1]
	} else if strings.Contains(domain, ":") {
		if strings.Count(domain, ":") == 1 {
			domain = strings.Split(domain, ":")[0]
		}
	}
	// DOMAIN处理结束

	capabilitiesCheck()
	// return

	if *dn42 {
		// 初始化配置
		config.InitConfig()
		*dataOrigin = "DN42"
		*disableMaptrace = true
	}

	/**
	 * 此处若使用goroutine同时运行ws的建立与nslookup，
	 * 会导致第一跳的IP信息无法获取，原因不明。
	 */
	//var wg sync.WaitGroup
	//wg.Add(2)
	//
	//go func() {
	//	defer wg.Done()
	//}()
	// MTR 使用 TUI 备用屏，必须在 wshandle.New() / GetFastIP 之前
	// 禁止彩色横幅输出，避免污染主终端历史。
	if effectiveMTR {
		util.SuppressFastIPOutput = true
	}

	var leoWs *wshandle.WsConn
	needsLeoWS := strings.EqualFold(*dataOrigin, "LEOMOEAPI")
	if needsLeoWS {
		if !strings.EqualFold(*powProvider, "api.nxtrace.org") {
			util.PowProviderParam = *powProvider
		}
		if util.EnvDataProvider != "" {
			*dataOrigin = util.EnvDataProvider
		}
		needsLeoWS = strings.EqualFold(*dataOrigin, "LEOMOEAPI")
		if needsLeoWS {
			leoWs = wshandle.New()
			if leoWs != nil {
				leoWs.Interrupt = make(chan os.Signal, 1)
				signal.Notify(leoWs.Interrupt, os.Interrupt)
			}
		}
	}
	if leoWs != nil {
		defer func() {
			if leoWs.Conn != nil {
				_ = leoWs.Conn.Close()
			}
		}()
	}

	if *from != "" {
		executeGlobalpingTraceroute(
			&trace.GlobalpingOptions{
				Target:  *str,
				From:    *from,
				IPv4:    *ipv4Only,
				IPv6:    *ipv6Only,
				TCP:     *tcp,
				UDP:     *udp,
				Port:    *port,
				Packets: *numMeasurements,
				MaxHops: *maxHops,

				DisableMaptrace: *disableMaptrace,
				DataOrigin:      *dataOrigin,

				TablePrint:   *tablePrint,
				ClassicPrint: *classicPrint,
				RawPrint:     *rawPrint,
				JSONPrint:    *jsonPrint,
			},
			&trace.Config{
				OSType:          OSType,
				DN42:            *dn42,
				NumMeasurements: *numMeasurements,
				Lang:            *lang,
				RDNS:            !*norDNS,
				AlwaysWaitRDNS:  *alwaysrDNS,
				IPGeoSource:     ipgeo.GetSource(*dataOrigin),
				Timeout:         time.Duration(*timeout) * time.Millisecond,
			},
		)
		return
	}

	var ip net.IP
	if *ipv6Only {
		ip, err = util.DomainLookUp(domain, "6", *dot, *jsonPrint)
	} else if *ipv4Only {
		ip, err = util.DomainLookUp(domain, "4", *dot, *jsonPrint)
	} else {
		ip, err = util.DomainLookUp(domain, "all", *dot, *jsonPrint)
	}

	if err != nil {
		if util.EnvDevMode {
			panic(err)
		}
		log.Fatal(err)
	}

	if *srcDev != "" {
		dev, devErr := net.InterfaceByName(*srcDev)
		if devErr != nil || dev == nil {
			fmt.Printf("无法找到网卡 %q: %v\n", *srcDev, devErr)
			os.Exit(1)
		}
		util.SrcDev = dev.Name
		if addrs, err := dev.Addrs(); err == nil {
			for _, addr := range addrs {
				ipNet, ok := addr.(*net.IPNet)
				if !ok {
					continue // 跳过非 *net.IPNet 类型（如 *net.IPAddr）
				}
				if (ipNet.IP.To4() == nil) == (ip.To4() == nil) {
					*srcAddr = ipNet.IP.String()
					// 检查是否是内网IP
					if !(net.ParseIP(*srcAddr).IsPrivate() ||
						net.ParseIP(*srcAddr).IsLoopback() ||
						net.ParseIP(*srcAddr).IsLinkLocalUnicast() ||
						net.ParseIP(*srcAddr).IsLinkLocalMulticast()) {
						// 若不是则跳出
						break
					}
				}
			}
		}
	}

	// 仅在使用 UDPv6 探测时，确保 UDP 负载长度 ≥ 2
	if *udp && util.IsIPv6(ip) && *packetSize < 2 {
		fmt.Println("UDPv6 模式下，数据包长度不能小于 2，已自动调整为 2")
		*packetSize = 2
	}

	if !*jsonPrint && !effectiveMTR {
		printer.PrintTraceRouteNav(ip, domain, *dataOrigin, *maxHops, *packetSize, *srcAddr, string(m))
	}

	util.SrcPort = *srcPort
	util.DstIP = ip.String()
	var conf = trace.Config{
		OSType:           OSType,
		ICMPMode:         *icmpMode,
		DN42:             *dn42,
		SrcAddr:          *srcAddr,
		SrcPort:          *srcPort,
		BeginHop:         *beginHop,
		DstIP:            ip,
		DstPort:          *port,
		MaxHops:          *maxHops,
		PacketInterval:   *packetInterval,
		TTLInterval:      *ttlInterval,
		NumMeasurements:  *numMeasurements,
		MaxAttempts:      *maxAttempts,
		ParallelRequests: *parallelRequests,
		Lang:             *lang,
		RDNS:             !*norDNS,
		AlwaysWaitRDNS:   *alwaysrDNS,
		IPGeoSource:      ipgeo.GetSource(*dataOrigin),
		Timeout:          time.Duration(*timeout) * time.Millisecond,
		PktSize:          *packetSize,
	}

	// --disable-mpls 需在 MTR 分支之前生效
	if *disableMPLS {
		util.DisableMPLS = true
	}

	// ── MTR 连续探测模式 ──
	if effectiveMTR {
		mtrMaxPerHop, mtrHopIntervalMs := deriveMTRProbeParams(
			effectiveReport,
			queriesExplicit,
			*numMeasurements,
			ttlTimeExplicit,
			*ttlInterval,
		)

		switch chooseMTRRunMode(effectiveMTRRaw, effectiveReport) {
		case mtrRunRaw:
			runMTRRaw(m, conf, mtrHopIntervalMs, mtrMaxPerHop, *dataOrigin)
		case mtrRunReport:
			runMTRReport(m, conf, mtrHopIntervalMs, mtrMaxPerHop, domain, *dataOrigin, effectiveWide, *showIPs)
		default:
			// -y/--ipinfo 仅 TUI 使用，此处校验
			if *ipInfoMode < 0 || *ipInfoMode > 4 {
				fmt.Fprintf(os.Stderr, "--ipinfo/-y 必须在 0-4 范围内，当前值: %d\n", *ipInfoMode)
				os.Exit(1)
			}
			runMTRTUI(m, conf, mtrHopIntervalMs, mtrMaxPerHop, domain, *dataOrigin, *showIPs, *ipInfoMode)
		}
		return
	}

	// 暂时弃用
	router := new(bool)
	*router = false
	if !*tablePrint {
		if *classicPrint {
			conf.RealtimePrinter = printer.ClassicPrinter
		} else if *rawPrint {
			conf.RealtimePrinter = printer.EasyPrinter
		} else {
			if *output {
				conf.RealtimePrinter = tracelog.RealtimePrinter
			} else if *router {
				conf.RealtimePrinter = printer.RealtimePrinterWithRouter
				fmt.Println("路由表数据源由 BGP.Tools 提供，在此特表感谢")
			} else {
				conf.RealtimePrinter = printer.RealtimePrinter
			}
		}
	}

	if *jsonPrint {
		conf.RealtimePrinter = nil
		conf.AsyncPrinter = nil
	}

	if util.Uninterrupted && *rawPrint {
		for {
			_, err := trace.Traceroute(m, conf)
			if err != nil {
				fmt.Println(err)
			}
		}
	}

	res, err := trace.Traceroute(m, conf)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			// 用户主动中断：跳过后续的正常收尾
			// os.Exit(130)
			fmt.Println(err)
		}
		return
	}

	if *tablePrint {
		printer.TracerouteTablePrinter(res)
	}

	if *routePath {
		r := reporter.New(res, ip.String())
		r.Print()
	}

	r, err := json.Marshal(res)
	if err != nil {
		fmt.Println(err)
		return
	}
	if !*disableMaptrace &&
		(util.StringInSlice(strings.ToUpper(*dataOrigin), []string{"LEOMOEAPI", "IPINFO", "IP-API.COM", "IPAPI.COM"})) {
		url, err := tracemap.GetMapUrl(string(r))
		if err != nil {
			fmt.Println(err)
			return
		}
		res.TraceMapUrl = url
		if !*jsonPrint {
			tracemap.PrintMapUrl(url)
		}
	}
	r, err = json.Marshal(res)
	if err != nil {
		fmt.Println(err)
		return
	}
	if *jsonPrint {
		fmt.Println(string(r))
	}
}

type mtrRunMode int

const (
	mtrRunTUI mtrRunMode = iota
	mtrRunReport
	mtrRunRaw
)

func chooseMTRRunMode(effectiveMTRRaw, effectiveReport bool) mtrRunMode {
	if effectiveMTRRaw {
		return mtrRunRaw
	}
	if effectiveReport {
		return mtrRunReport
	}
	return mtrRunTUI
}

// deriveMTRProbeParams computes per-hop scheduling parameters for MTR.
//
// maxPerHop priority: explicit -q > report default 10 > TUI/raw default 0 (unlimited).
// hopIntervalMs priority: explicit -i > default 1000.
func deriveMTRProbeParams(
	effectiveReport, queriesExplicit bool, numMeasurements int,
	ttlTimeExplicit bool, ttlInterval int,
) (maxPerHop int, hopIntervalMs int) {
	// maxPerHop
	if queriesExplicit {
		maxPerHop = numMeasurements
	} else if effectiveReport {
		maxPerHop = 10 // report 默认 10
	} else {
		maxPerHop = 0 // TUI/raw → 无限
	}

	// hopIntervalMs
	if ttlTimeExplicit {
		hopIntervalMs = ttlInterval
	} else {
		hopIntervalMs = 1000
	}
	return
}

// deriveMTRRoundParams is the legacy round-based parameter derivation.
// Kept for backward compatibility (Web MTR).
func deriveMTRRoundParams(effectiveReport, queriesExplicit bool, numMeasurements int, ttlTimeExplicit bool, ttlInterval int) (maxRounds int, intervalMs int) {
	if effectiveReport {
		if queriesExplicit {
			maxRounds = numMeasurements
		} else {
			maxRounds = 10 // report 默认 10 轮
		}
	} else if queriesExplicit {
		maxRounds = numMeasurements
	} else {
		maxRounds = 0 // 非 report → 无限
	}

	if ttlTimeExplicit {
		intervalMs = ttlInterval
	} else {
		intervalMs = 1000 // MTR 默认 1000ms
	}
	return
}

func capabilitiesCheck() {
	// Windows 判断放在前面，防止遇到一些奇奇怪怪的问题
	if runtime.GOOS == "windows" {
		// Running on Windows, skip checking capabilities
		return
	}

	uid := os.Getuid()
	if uid == 0 {
		// Running as root, skip checking capabilities
		return
	}

	/***
	* 检查当前进程是否有两个关键的权限
	==== 看不到我 ====
	* 没办法啦
	* 自己之前承诺的坑补全篇
	* 被迫填坑系列 qwq
	==== 看不到我 ====
	***/

	// NewPid 已经被废弃了，这里改用 NewPid2 方法
	caps, err := capability.NewPid2(0)
	if err != nil {
		// 判断是否为macOS
		if runtime.GOOS == "darwin" {
			// macOS下报错有问题
		} else {
			fmt.Println(err)
		}
		return
	}

	// load 获取全部的 caps 信息
	err = caps.Load()
	if err != nil {
		fmt.Println(err)
		return
	}

	// 判断一下权限有木有
	if caps.Get(capability.EFFECTIVE, capability.CAP_NET_RAW) && caps.Get(capability.EFFECTIVE, capability.CAP_NET_ADMIN) {
		// 有权限啦
		return
	} else {
		// 没权限啦
		fmt.Println("您正在以普通用户权限运行 NextTrace，但 NextTrace 未被赋予监听网络套接字的ICMP消息包、修改IP头信息（TTL）等路由跟踪所需的权限")
		fmt.Println("请使用管理员用户执行 `sudo setcap cap_net_raw,cap_net_admin+eip ${your_nexttrace_path}/nexttrace` 命令，赋予相关权限后再运行~")
		fmt.Println("什么？为什么 ping 普通用户执行不要 root 权限？因为这些工具在管理员安装时就已经被赋予了一些必要的权限，具体请使用 `getcap /usr/bin/ping` 查看")
	}
}

func executeGlobalpingTraceroute(opts *trace.GlobalpingOptions, config *trace.Config) {
	res, measurement, err := trace.GlobalpingTraceroute(opts, config)
	if err != nil {
		fmt.Println(err)
		return
	}

	if !opts.DisableMaptrace &&
		(util.StringInSlice(strings.ToUpper(opts.DataOrigin), []string{"LEOMOEAPI", "IPINFO", "IP-API.COM", "IPAPI.COM"})) {
		r, err := json.Marshal(res)
		if err != nil {
			fmt.Println(err)
			return
		}
		url, err := tracemap.GetMapUrl(string(r))
		if err != nil {
			fmt.Println(err)
			return
		}
		res.TraceMapUrl = url
	}

	if opts.JSONPrint {
		r, err := json.Marshal(res)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(string(r))
		return
	}

	if measurement == nil || len(measurement.Results) == 0 {
		fmt.Println("Globalping 未返回可用的探测结果，已跳过输出。")
		return
	}

	fmt.Fprintln(color.Output, color.New(color.FgGreen, color.Bold).Sprintf("> %s", trace.GlobalpingFormatLocation(&measurement.Results[0])))

	if opts.TablePrint {
		printer.TracerouteTablePrinter(res)
	} else {
		for i := range res.Hops {
			if opts.ClassicPrint {
				printer.ClassicPrinter(res, i)
			} else if opts.RawPrint {
				printer.EasyPrinter(res, i)
			} else {
				printer.RealtimePrinter(res, i)
			}
		}
	}

	if res.TraceMapUrl != "" {
		tracemap.PrintMapUrl(res.TraceMapUrl)
	}
}
