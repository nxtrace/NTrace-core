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

	"github.com/akamensky/argparse"
	"github.com/syndtr/gocapability/capability"
	"github.com/xgadget-lab/nexttrace/config"
	fastTrace "github.com/xgadget-lab/nexttrace/fast_trace"
	"github.com/xgadget-lab/nexttrace/ipgeo"
	"github.com/xgadget-lab/nexttrace/printer"
	"github.com/xgadget-lab/nexttrace/reporter"
	"github.com/xgadget-lab/nexttrace/trace"
	"github.com/xgadget-lab/nexttrace/tracelog"
	"github.com/xgadget-lab/nexttrace/tracemap"
	"github.com/xgadget-lab/nexttrace/util"
	"github.com/xgadget-lab/nexttrace/wshandle"
)

func Excute() {
	parser := argparse.NewParser("nexttrace", "An open source visual route tracking CLI tool")
	// Create string flag
	ipv4Only := parser.Flag("4", "ipv4", &argparse.Options{Help: "Use IPv4 only"})
	ipv6Only := parser.Flag("6", "ipv6", &argparse.Options{Help: "Use IPv6 only"})
	tcp := parser.Flag("T", "tcp", &argparse.Options{Help: "Use TCP SYN for tracerouting (default port is 80)"})
	udp := parser.Flag("U", "udp", &argparse.Options{Help: "Use UDP SYN for tracerouting (default port is 53)"})
	fast_trace := parser.Flag("F", "fast-trace", &argparse.Options{Help: "One-Key Fast Trace to China ISPs"})
	port := parser.Int("p", "port", &argparse.Options{Help: "Set the destination port to use. It is either initial udp port value for \"default\"" +
		"method (incremented by each probe, default is 33434), or initial seq for \"icmp\" (incremented as well, default from 1), or some constant" +
		"destination port for other methods (with default of 80 for \"tcp\", 53 for \"udp\", etc.)"})
	numMeasurements := parser.Int("q", "queries", &argparse.Options{Default: 3, Help: "Set the number of probes per each hop"})
	parallelRequests := parser.Int("", "parallel-requests", &argparse.Options{Default: 18, Help: "Set ParallelRequests number. It should be 1 when there is a multi-routing"})
	maxHops := parser.Int("m", "max-hops", &argparse.Options{Default: 30, Help: "Set the max number of hops (max TTL to be reached)"})
	dataOrigin := parser.Selector("d", "data-provider", []string{"Ip2region", "ip2region", "IP.SB", "ip.sb", "IPInfo", "ipinfo", "IPInsight", "ipinsight", "IPAPI.com", "ip-api.com", "IPInfoLocal", "ipinfolocal", "chunzhen", "LeoMoeAPI", "leomoeapi", "disable-geoip"}, &argparse.Options{Default: "LeoMoeAPI",
		Help: "Choose IP Geograph Data Provider [IP.SB, IPInfo, IPInsight, IP-API.com, Ip2region, IPInfoLocal, CHUNZHEN, disable-geoip]"})
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
	ver := parser.Flag("v", "version", &argparse.Options{Help: "Print version info and exit"})
	src_addr := parser.String("s", "source", &argparse.Options{Help: "Use source src_addr for outgoing packets"})
	src_dev := parser.String("D", "dev", &argparse.Options{Help: "Use the following Network Devices as the source address in outgoing packets"})
	router := parser.Flag("R", "route", &argparse.Options{Help: "Show Routing Table [Provided By BGP.Tools]"})
	packet_interval := parser.Int("z", "send-time", &argparse.Options{Default: 100, Help: "Set the time interval for sending every packet. Useful when some routers use rate-limit for ICMP messages"})
	ttl_interval := parser.Int("i", "ttl-time", &argparse.Options{Default: 500, Help: "Set the time interval for sending packets groups by TTL. Useful when some routers use rate-limit for ICMP messages"})
	str := parser.StringPositional(&argparse.Options{Help: "IP Address or domain name"})
	dot := parser.Selector("", "dot-server", []string{"dnssb", "aliyun", "dnspod", "google", "cloudflare"}, &argparse.Options{
		Help: "Use DoT Server for DNS Parse [dnssb, aliyun, dnspod, google, cloudflare]"})
	lang := parser.Selector("g", "language", []string{"en", "cn"}, &argparse.Options{Default: "cn",
		Help: "Choose the language for displaying [en, cn]"})

	err := parser.Parse(os.Args)
	if err != nil {
		// In case of error print error and print usage
		// This can also be done by passing -h or --help flags
		fmt.Print(parser.Usage(err))
		return
	}
	if !*jsonPrint {
		printer.Version()
	}
	if *ver {
		printer.CopyRight()
		os.Exit(0)
	}

	domain := *str

	if *port == 0 {
		*port = 80
	}

	if *fast_trace {
		fastTrace.FastTest(*tcp, *output)
		if *output {
			fmt.Println("您的追踪日志已经存放在 /tmp/trace.log 中")
		}

		os.Exit(0)
	}

	if domain == "" {
		fmt.Print(parser.Usage(err))
		return
	}

	if strings.Contains(domain, "/") {
		domain = strings.Split(domain, "/")[2]
	}

	if strings.Contains(domain, "]") {
		domain = strings.Split(strings.Split(domain, "]")[0], "[")[1]
	} else if strings.Contains(domain, ":") {
		if strings.Count(domain, ":") == 1 {
			domain = strings.Split(domain, ":")[0]
		}
	}

	capabilities_check()
	// return

	var ip net.IP

	if runtime.GOOS == "windows" && (*tcp || *udp) {
		fmt.Println("NextTrace 基于 Windows 的路由跟踪还在早期开发阶段，目前还存在诸多问题，TCP/UDP SYN 包请求可能不能正常运行")
	}

	if *udp {
		if *ipv6Only {
			fmt.Println("[Info] IPv6 UDP Traceroute is not supported right now.")
			os.Exit(0)
		}
		ip = util.DomainLookUp(domain, "4", *dot, *jsonPrint)
	} else {
		if *ipv6Only {
			ip = util.DomainLookUp(domain, "6", *dot, *jsonPrint)
		} else if *ipv4Only {
			ip = util.DomainLookUp(domain, "4", *dot, *jsonPrint)
		} else {
			ip = util.DomainLookUp(domain, "all", *dot, *jsonPrint)
		}
	}

	if *src_dev != "" {
		dev, _ := net.InterfaceByName(*src_dev)

		if addrs, err := dev.Addrs(); err == nil {
			for _, addr := range addrs {
				if (addr.(*net.IPNet).IP.To4() == nil) == (ip.To4() == nil) {
					*src_addr = addr.(*net.IPNet).IP.String()
				}
			}
		}
	}

	if *dn42 {
		// 初始化配置
		config.InitConfig()
		*dataOrigin = "DN42"
		*disableMaptrace = true
	}

	if strings.ToUpper(*dataOrigin) == "LEOMOEAPI" {
		val, ok := os.LookupEnv("NEXTTRACE_DATAPROVIDER")
		if ok {
			*dataOrigin = val
		} else {
			w := wshandle.New()
			w.Interrupt = make(chan os.Signal, 1)
			signal.Notify(w.Interrupt, os.Interrupt)
			defer func() {
				w.Conn.Close()
			}()
		}
	}

	if !*jsonPrint {
		printer.PrintTraceRouteNav(ip, domain, *dataOrigin, *maxHops)
	}

	var m trace.Method = ""

	switch {
	case *tcp:
		m = trace.TCPTrace
	case *udp:
		m = trace.UDPTrace
	default:
		m = trace.ICMPTrace
	}

	if !*tcp && *port == 80 {
		*port = 53
	}

	var conf = trace.Config{
		DN42:             *dn42,
		SrcAddr:          *src_addr,
		BeginHop:         *beginHop,
		DestIP:           ip,
		DestPort:         *port,
		MaxHops:          *maxHops,
		PacketInterval:   *packet_interval,
		TTLInterval:      *ttl_interval,
		NumMeasurements:  *numMeasurements,
		ParallelRequests: *parallelRequests,
		Lang:             *lang,
		RDns:             !*noRdns,
		AlwaysWaitRDNS:   *alwaysRdns,
		IPGeoSource:      ipgeo.GetSource(*dataOrigin),
		Timeout:          1 * time.Second,
	}

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
	if !*disableMaptrace && strings.ToUpper(*dataOrigin) == "LEOMOEAPI" {
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

func capabilities_check() {

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
