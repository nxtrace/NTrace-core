package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/akamensky/argparse"
	"github.com/nxtrace/NTrace-core/config"
	fastTrace "github.com/nxtrace/NTrace-core/fast_trace"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/reporter"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/tracelog"
	"github.com/nxtrace/NTrace-core/tracemap"
	"github.com/nxtrace/NTrace-core/util"
	"github.com/nxtrace/NTrace-core/wshandle"
	"github.com/syndtr/gocapability/capability"
)

func Excute() {
	parser := argparse.NewParser("nexttrace", "An open source visual route tracking CLI tool")
	// Create string flag
	ipv4Only := parser.Flag("4", "ipv4", &argparse.Options{Help: "Use IPv4 only"})
	ipv6Only := parser.Flag("6", "ipv6", &argparse.Options{Help: "Use IPv6 only"})
	tcp := parser.Flag("T", "tcp", &argparse.Options{Help: "Use TCP SYN for tracerouting (default port is 80)"})
	udp := parser.Flag("U", "udp", &argparse.Options{Help: "Use UDP SYN for tracerouting (default port is 33494)"})
	fast_trace := parser.Flag("F", "fast-trace", &argparse.Options{Help: "One-Key Fast Trace to China ISPs"})
	port := parser.Int("p", "port", &argparse.Options{Help: "Set the destination port to use. With default of 80 for \"tcp\", 33494 for \"udp\"", Default: 80})
	numMeasurements := parser.Int("q", "queries", &argparse.Options{Default: 3, Help: "Set the number of probes per each hop"})
	parallelRequests := parser.Int("", "parallel-requests", &argparse.Options{Default: 18, Help: "Set ParallelRequests number. It should be 1 when there is a multi-routing"})
	maxHops := parser.Int("m", "max-hops", &argparse.Options{Default: 30, Help: "Set the max number of hops (max TTL to be reached)"})
	dataOrigin := parser.Selector("d", "data-provider", []string{"Ip2region", "ip2region", "IP.SB", "ip.sb", "IPInfo", "ipinfo", "IPInsight", "ipinsight", "IPAPI.com", "ip-api.com", "IPInfoLocal", "ipinfolocal", "chunzhen", "LeoMoeAPI", "leomoeapi", "disable-geoip"}, &argparse.Options{Default: "LeoMoeAPI",
		Help: "Choose IP Geograph Data Provider [IP.SB, IPInfo, IPInsight, IP-API.com, Ip2region, IPInfoLocal, CHUNZHEN, disable-geoip]"})
	powProvider := parser.Selector("", "pow-provider", []string{"api.nxtrace.org", "sakura"}, &argparse.Options{Default: "api.nxtrace.org",
		Help: "Choose PoW Provider [api.nxtrace.org, sakura] For China mainland users, please use sakura"})
	noRdns := parser.Flag("n", "no-rdns", &argparse.Options{Help: "Do not resolve IP addresses to their domain names"})
	alwaysRdns := parser.Flag("a", "always-rdns", &argparse.Options{Help: "Always resolve IP addresses to their domain names"})
	routePath := parser.Flag("P", "route-path", &argparse.Options{Help: "Print traceroute hop path by ASN and location"})
	report := parser.Flag("r", "report", &argparse.Options{Help: "output using report mode"})
	dn42 := parser.Flag("", "dn42", &argparse.Options{Help: "DN42 Mode"})
	output := parser.Flag("o", "output", &argparse.Options{Help: "Write trace result to file (RealTimePrinter ONLY)"})
	tablePrint := parser.Flag("t", "table", &argparse.Options{Help: "Output trace results as table"})
	rawPrint := parser.Flag("", "raw", &argparse.Options{Help: "An Output Easy to Parse"})
	jsonPrint := parser.Flag("j", "json", &argparse.Options{Help: "Output trace results as JSON"})
	classicPrint := parser.Flag("c", "classic", &argparse.Options{Help: "Classic Output trace results like BestTrace"})
	beginHop := parser.Int("f", "first", &argparse.Options{Default: 1, Help: "Start from the first_ttl hop (instead from 1)"})
	disableMaptrace := parser.Flag("M", "map", &argparse.Options{Help: "Disable Print Trace Map"})
	disableMPLS := parser.Flag("e", "disable-mpls", &argparse.Options{Help: "Disable MPLS"})
	ver := parser.Flag("v", "version", &argparse.Options{Help: "Print version info and exit"})
	srcAddr := parser.String("s", "source", &argparse.Options{Help: "Use source src_addr for outgoing packets"})
	srcDev := parser.String("D", "dev", &argparse.Options{Help: "Use the following Network Devices as the source address in outgoing packets"})
	//router := parser.Flag("R", "route", &argparse.Options{Help: "Show Routing Table [Provided By BGP.Tools]"})
	packetInterval := parser.Int("z", "send-time", &argparse.Options{Default: 50, Help: "Set how many [milliseconds] between sending each packet.. Useful when some routers use rate-limit for ICMP messages"})
	ttlInterval := parser.Int("i", "ttl-time", &argparse.Options{Default: 50, Help: "Set how many [milliseconds] between sending packets groups by TTL. Useful when some routers use rate-limit for ICMP messages"})
	timeout := parser.Int("", "timeout", &argparse.Options{Default: 1000, Help: "The number of [milliseconds] to keep probe sockets open before giving up on the connection."})
	packetSize := parser.Int("", "psize", &argparse.Options{Default: 52, Help: "Set the payload size"})
	str := parser.StringPositional(&argparse.Options{Help: "IP Address or domain name"})
	dot := parser.Selector("", "dot-server", []string{"dnssb", "aliyun", "dnspod", "google", "cloudflare"}, &argparse.Options{
		Help: "Use DoT Server for DNS Parse [dnssb, aliyun, dnspod, google, cloudflare]"})
	lang := parser.Selector("g", "language", []string{"en", "cn"}, &argparse.Options{Default: "cn",
		Help: "Choose the language for displaying [en, cn]"})
	file := parser.String("", "file", &argparse.Options{Help: "Read IP Address or domain name from file"})
	nocolor := parser.Flag("C", "nocolor", &argparse.Options{Help: "Disable Colorful Output"})
	dontFragment := parser.Flag("", "dont-fragment", &argparse.Options{Default: false, Help: "Set the Don't Fragment bit (IPv4 TCP only)"})

	err := parser.Parse(os.Args)
	if err != nil {
		// In case of error print error and print usage
		// This can also be done by passing -h or --help flags
		fmt.Print(parser.Usage(err))
		return
	}

	if *nocolor {
		color.NoColor = true
	} else {
		color.NoColor = false
	}

	if !*jsonPrint {
		printer.Version()
	}

	if *ver {
		printer.CopyRight()
		os.Exit(0)
	}

	if !*tcp && *port == 80 {
		*port = 33494
	}

	domain := *str

	var m trace.Method

	switch {
	case *tcp:
		m = trace.TCPTrace
	case *udp:
		m = trace.UDPTrace
	default:
		m = trace.ICMPTrace
	}

	if *fast_trace || *file != "" {
		var paramsFastTrace = fastTrace.ParamsFastTrace{
			SrcDev:         *srcDev,
			SrcAddr:        *srcAddr,
			DestPort:       *port,
			BeginHop:       *beginHop,
			MaxHops:        *maxHops,
			RDns:           !*noRdns,
			AlwaysWaitRDNS: *alwaysRdns,
			Lang:           *lang,
			PktSize:        *packetSize,
			Timeout:        time.Duration(*timeout) * time.Millisecond,
			File:           *file,
			DontFragment:   *dontFragment,
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

	var ip net.IP

	if runtime.GOOS == "windows" && (*tcp || *udp) {
		fmt.Println("NextTrace 基于 Windows 的路由跟踪还在早期开发阶段，目前还存在诸多问题，TCP/UDP SYN 包请求可能不能正常运行")
	}

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
	if strings.ToUpper(*dataOrigin) == "LEOMOEAPI" {
		val, ok := os.LookupEnv("NEXTTRACE_DATAPROVIDER")
		if strings.ToUpper(*powProvider) != "API.NXTRACE.ORG" {
			util.PowProviderParam = *powProvider
		}
		if ok {
			*dataOrigin = val
		} else {
			w := wshandle.New()
			w.Interrupt = make(chan os.Signal, 1)
			signal.Notify(w.Interrupt, os.Interrupt)
			defer func() {
				if w.Conn != nil {
					w.Conn.Close()
				}
			}()
		}
	}
	//}()
	//
	//go func() {
	//	defer wg.Done()
	if *udp {
		if *ipv6Only {
			fmt.Println("[Info] IPv6 UDP Traceroute is not supported right now.")
			os.Exit(0)
		}
		ip, err = util.DomainLookUp(domain, "4", *dot, *jsonPrint)
	} else {
		if *ipv6Only {
			ip, err = util.DomainLookUp(domain, "6", *dot, *jsonPrint)
		} else if *ipv4Only {
			ip, err = util.DomainLookUp(domain, "4", *dot, *jsonPrint)
		} else {
			ip, err = util.DomainLookUp(domain, "all", *dot, *jsonPrint)
		}
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	//}()
	//
	//wg.Wait()

	if *srcDev != "" {
		dev, _ := net.InterfaceByName(*srcDev)
		if addrs, err := dev.Addrs(); err == nil {
			for _, addr := range addrs {
				if (addr.(*net.IPNet).IP.To4() == nil) == (ip.To4() == nil) {
					*srcAddr = addr.(*net.IPNet).IP.String()
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

	if !*jsonPrint {
		printer.PrintTraceRouteNav(ip, domain, *dataOrigin, *maxHops, *packetSize, *srcAddr, string(m))
	}

	util.DestIP = ip.String()
	var conf = trace.Config{
		DN42:             *dn42,
		SrcAddr:          *srcAddr,
		BeginHop:         *beginHop,
		DestIP:           ip,
		DestPort:         *port,
		MaxHops:          *maxHops,
		PacketInterval:   *packetInterval,
		TTLInterval:      *ttlInterval,
		NumMeasurements:  *numMeasurements,
		ParallelRequests: *parallelRequests,
		Lang:             *lang,
		RDns:             !*noRdns,
		AlwaysWaitRDNS:   *alwaysRdns,
		IPGeoSource:      ipgeo.GetSource(*dataOrigin),
		Timeout:          time.Duration(*timeout) * time.Millisecond,
		PktSize:          *packetSize,
		DontFragment:     *dontFragment,
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
	} else {
		if !*report {
			conf.AsyncPrinter = printer.TracerouteTablePrinter
		}
	}

	if *jsonPrint {
		conf.RealtimePrinter = nil
		conf.AsyncPrinter = nil
	}

	if util.Uninterrupted != "" && *rawPrint {
		for {
			_, err := trace.Traceroute(m, conf)
			if err != nil {
				fmt.Println(err)
			}
		}
	}

	if *disableMPLS {
		util.DisableMPLS = "1"
	}

	res, err := trace.Traceroute(m, conf)

	if err != nil {
		log.Fatalln(err)
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
		(util.StringInSlice(strings.ToUpper(*dataOrigin), []string{"LEOMOEAPI", "IPINFO", "IPINFO", "IP-API.COM", "IPAPI.COM"})) {
		url, err := tracemap.GetMapUrl(string(r))
		if err != nil {
			log.Fatalln(err)
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
