package fastTrace

import (
	"bufio"
	"fmt"
	"github.com/fatih/color"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/tracelog"
	"github.com/nxtrace/NTrace-core/util"
	"github.com/nxtrace/NTrace-core/wshandle"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"
)

type FastTracer struct {
	TracerouteMethod trace.Method
	ParamsFastTrace  ParamsFastTrace
}

type ParamsFastTrace struct {
	SrcDev         string
	SrcAddr        string
	DestPort       int
	BeginHop       int
	MaxHops        int
	RDns           bool
	AlwaysWaitRDNS bool
	Lang           string
	PktSize        int
	Timeout        time.Duration
	File           string
	DontFragment   bool
	Dot            string
}

type IpListElement struct {
	Ip       string
	Desc     string
	Version4 bool // true for IPv4, false for IPv6
}

var oe = false

func (f *FastTracer) tracert(location string, ispCollection ISPCollection) {
	fmt.Fprintf(color.Output, "%s\n", color.New(color.FgYellow, color.Bold).Sprintf("『%s %s 』", location, ispCollection.ISPName))
	fmt.Printf("traceroute to %s, %d hops max, %d byte packets, %s mode\n", ispCollection.IP, f.ParamsFastTrace.MaxHops, f.ParamsFastTrace.PktSize, strings.ToUpper(string(f.TracerouteMethod)))

	// ip, err := util.DomainLookUp(ispCollection.IP, "4", "", true)
	ip, err := util.DomainLookUp(ispCollection.IP, "4", f.ParamsFastTrace.Dot, true)
	if err != nil {
		log.Fatal(err)
	}
	var conf = trace.Config{
		BeginHop:         f.ParamsFastTrace.BeginHop,
		DestIP:           ip,
		DestPort:         f.ParamsFastTrace.DestPort,
		MaxHops:          f.ParamsFastTrace.MaxHops,
		NumMeasurements:  3,
		ParallelRequests: 18,
		RDns:             f.ParamsFastTrace.RDns,
		AlwaysWaitRDNS:   f.ParamsFastTrace.AlwaysWaitRDNS,
		PacketInterval:   100,
		TTLInterval:      500,
		IPGeoSource:      ipgeo.GetSource("LeoMoeAPI"),
		Timeout:          f.ParamsFastTrace.Timeout,
		SrcAddr:          f.ParamsFastTrace.SrcAddr,
		PktSize:          f.ParamsFastTrace.PktSize,
		Lang:             f.ParamsFastTrace.Lang,
		DontFragment:     f.ParamsFastTrace.DontFragment,
	}

	if oe {
		fp, err := os.OpenFile("/tmp/trace.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm)
		if err != nil {
			return
		}
		defer func(fp *os.File) {
			err := fp.Close()
			if err != nil {
				log.Fatal(err)
			}
		}(fp)

		log.SetOutput(fp)
		log.SetFlags(0)
		log.Printf("『%s %s 』\n", location, ispCollection.ISPName)
		log.Printf("traceroute to %s, %d hops max, %d byte packets, %s mode\n", ispCollection.IP, f.ParamsFastTrace.MaxHops, f.ParamsFastTrace.PktSize, strings.ToUpper(string(f.TracerouteMethod)))
		conf.RealtimePrinter = tracelog.RealtimePrinter
	} else {
		conf.RealtimePrinter = printer.RealtimePrinter
	}

	_, err = trace.Traceroute(f.TracerouteMethod, conf)

	if err != nil {
		log.Fatal(err)
	}
	fmt.Println()
}

func FastTest(traceMode trace.Method, outEnable bool, paramsFastTrace ParamsFastTrace) {
	// tm means tcp mode
	var c string
	oe = outEnable

	if paramsFastTrace.File != "" {
		testFile(paramsFastTrace, traceMode)
		return
	}

	fmt.Println("Hi，欢迎使用 Fast Trace 功能，请注意 Fast Trace 功能只适合新手使用\n因为国内网络复杂，我们设置的测试目标有限，建议普通用户自测以获得更加精准的路由情况")
	fmt.Println("请您选择要测试的 IP 类型\n1. IPv4\n2. IPv6")
	fmt.Print("请选择选项：")
	_, err := fmt.Scanln(&c)
	if err != nil {
		c = "1"
	}
	if c == "2" {
		if paramsFastTrace.SrcDev != "" {
			dev, _ := net.InterfaceByName(paramsFastTrace.SrcDev)
			if addrs, err := dev.Addrs(); err == nil {
				for _, addr := range addrs {
					if (addr.(*net.IPNet).IP.To4() == nil) == true {
						paramsFastTrace.SrcAddr = addr.(*net.IPNet).IP.String()
						// 检查是否是内网IP
						if !(net.ParseIP(paramsFastTrace.SrcAddr).IsPrivate() ||
							net.ParseIP(paramsFastTrace.SrcAddr).IsLoopback() ||
							net.ParseIP(paramsFastTrace.SrcAddr).IsLinkLocalUnicast() ||
							net.ParseIP(paramsFastTrace.SrcAddr).IsLinkLocalMulticast()) {
							// 若不是则跳出
							break
						}
					}
				}
			}
		}
		FastTestv6(traceMode, outEnable, paramsFastTrace)
		return
	}
	if paramsFastTrace.SrcDev != "" {
		dev, _ := net.InterfaceByName(paramsFastTrace.SrcDev)
		if addrs, err := dev.Addrs(); err == nil {
			for _, addr := range addrs {
				if (addr.(*net.IPNet).IP.To4() == nil) == false {
					paramsFastTrace.SrcAddr = addr.(*net.IPNet).IP.String()
					// 检查是否是内网IP
					if !(net.ParseIP(paramsFastTrace.SrcAddr).IsPrivate() ||
						net.ParseIP(paramsFastTrace.SrcAddr).IsLoopback() ||
						net.ParseIP(paramsFastTrace.SrcAddr).IsLinkLocalUnicast() ||
						net.ParseIP(paramsFastTrace.SrcAddr).IsLinkLocalMulticast()) {
						// 若不是则跳出
						break
					}
				}
			}
		}
	}

	fmt.Println("您想测试哪些ISP的路由？\n1. 北京三网快速测试\n2. 全国电信\n3. 全国联通\n4. 全国移动\n5. 全国教育网\n6. 全国五网")
	fmt.Print("请选择选项：")
	_, err = fmt.Scanln(&c)
	if err != nil {
		c = "1"
	}

	ft := FastTracer{
		ParamsFastTrace: paramsFastTrace,
	}

	// 建立 WebSocket 连接
	w := wshandle.New()
	w.Interrupt = make(chan os.Signal, 1)
	signal.Notify(w.Interrupt, os.Interrupt)
	defer func() {
		w.Conn.Close()
	}()

	switch traceMode {
	case trace.ICMPTrace:
		ft.TracerouteMethod = trace.ICMPTrace
	case trace.TCPTrace:
		ft.TracerouteMethod = trace.TCPTrace
	case trace.UDPTrace:
		ft.TracerouteMethod = trace.UDPTrace
	}

	switch c {
	case "1":
		ft.testFast()
	case "2":
		ft.testCT()
	case "3":
		ft.testCU()
	case "4":
		ft.testCM()
	case "5":
		ft.testEDU()
	case "6":
		ft.testAll()
	default:
		ft.testFast()
	}
}

func testFile(paramsFastTrace ParamsFastTrace, traceMode trace.Method) {
	// 建立 WebSocket 连接
	w := wshandle.New()
	w.Interrupt = make(chan os.Signal, 1)
	signal.Notify(w.Interrupt, os.Interrupt)
	defer func() {
		w.Conn.Close()
	}()

	var tracerouteMethod trace.Method
	switch traceMode {
	case trace.ICMPTrace:
		tracerouteMethod = trace.ICMPTrace
	case trace.TCPTrace:
		tracerouteMethod = trace.TCPTrace
	case trace.UDPTrace:
		tracerouteMethod = trace.UDPTrace
	}

	filePath := paramsFastTrace.File
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(file)
	var ipList []IpListElement
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)

		var ip, desc string
		if len(parts) == 2 {
			ip = parts[0]
			desc = parts[1]
		} else if len(parts) == 1 {
			ip = parts[0]
			desc = ip // Set the description to the IP if no description is provided
		} else {
			fmt.Printf("Ignoring invalid line: %s\n", line)
			continue
		}

		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			netIp, err := util.DomainLookUp(ip, "all", "", true)
			if err != nil {
				fmt.Printf("Ignoring invalid IP: %s\n", ip)
				continue
			}
			if len(parts) == 1 {
				desc = ip
			}
			ip = netIp.String()
		}

		ipElem := IpListElement{
			Ip:       ip,
			Desc:     desc,
			Version4: strings.Contains(ip, "."),
		}

		ipList = append(ipList, ipElem)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading file:", err)
	}

	for _, ip := range ipList {
		fmt.Fprintf(color.Output, "%s\n",
			color.New(color.FgYellow, color.Bold).Sprint("『 "+ip.Desc+"』"),
		)
		if util.EnableHidDstIP == "" {
			fmt.Printf("traceroute to %s, %d hops max, %d bytes payload, %s mode\n", ip.Ip, paramsFastTrace.MaxHops, paramsFastTrace.PktSize, strings.ToUpper(string(tracerouteMethod)))
		} else {
			fmt.Printf("traceroute to %s, %d hops max, %d bytes payload, %s mode\n", util.HideIPPart(ip.Ip), paramsFastTrace.MaxHops, paramsFastTrace.PktSize, strings.ToUpper(string(tracerouteMethod)))
		}
		var srcAddr string
		if ip.Version4 {
			if paramsFastTrace.SrcDev != "" {
				dev, _ := net.InterfaceByName(paramsFastTrace.SrcDev)
				if addrs, err := dev.Addrs(); err == nil {
					for _, addr := range addrs {
						if (addr.(*net.IPNet).IP.To4() == nil) == false {
							srcAddr = addr.(*net.IPNet).IP.String()
							// 检查是否是内网IP
							if !(net.ParseIP(srcAddr).IsPrivate() ||
								net.ParseIP(srcAddr).IsLoopback() ||
								net.ParseIP(srcAddr).IsLinkLocalUnicast() ||
								net.ParseIP(srcAddr).IsLinkLocalMulticast()) {
								// 若不是则跳出
								break
							}
						}
					}
				}
			}
		} else {
			if paramsFastTrace.SrcDev != "" {
				dev, _ := net.InterfaceByName(paramsFastTrace.SrcDev)
				if addrs, err := dev.Addrs(); err == nil {
					for _, addr := range addrs {
						if (addr.(*net.IPNet).IP.To4() == nil) == true {
							srcAddr = addr.(*net.IPNet).IP.String()
							// 检查是否是内网IP
							if !(net.ParseIP(srcAddr).IsPrivate() ||
								net.ParseIP(srcAddr).IsLoopback() ||
								net.ParseIP(srcAddr).IsLinkLocalUnicast() ||
								net.ParseIP(srcAddr).IsLinkLocalMulticast()) {
								// 若不是则跳出
								break
							}
						}
					}
				}
			}
		}

		var conf = trace.Config{
			BeginHop:         paramsFastTrace.BeginHop,
			DestIP:           net.ParseIP(ip.Ip),
			DestPort:         paramsFastTrace.DestPort,
			MaxHops:          paramsFastTrace.MaxHops,
			NumMeasurements:  3,
			ParallelRequests: 18,
			RDns:             paramsFastTrace.RDns,
			AlwaysWaitRDNS:   paramsFastTrace.AlwaysWaitRDNS,
			PacketInterval:   100,
			TTLInterval:      500,
			IPGeoSource:      ipgeo.GetSource("LeoMoeAPI"),
			Timeout:          paramsFastTrace.Timeout,
			SrcAddr:          srcAddr,
			PktSize:          paramsFastTrace.PktSize,
			Lang:             paramsFastTrace.Lang,
		}

		if oe {
			fp, err := os.OpenFile("/tmp/trace.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm)
			if err != nil {
				return
			}
			log.SetOutput(fp)
			log.SetFlags(0)
			log.Printf("『%s』\n", ip.Desc)
			log.Printf("traceroute to %s, %d hops max, %d byte packets, %s mode\n", ip.Ip, paramsFastTrace.MaxHops, paramsFastTrace.PktSize, strings.ToUpper(string(tracerouteMethod)))
			conf.RealtimePrinter = tracelog.RealtimePrinter
			err = fp.Close()
			if err != nil {
				log.Fatal(err)
			}
		} else {
			conf.RealtimePrinter = printer.RealtimePrinter
		}

		_, err := trace.Traceroute(tracerouteMethod, conf)
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println()
	}

}

func (f *FastTracer) testAll() {
	f.testCT()
	println()
	f.testCU()
	println()
	f.testCM()
	println()
	f.testEDU()
}

func (f *FastTracer) testCT() {
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CT163)
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CTCN2)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CT163)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CTCN2)
	f.tracert(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CT163)
	f.tracert(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CTCN2)
	f.tracert(TestIPsCollection.Hangzhou.Location, TestIPsCollection.Hangzhou.CT163)
}

func (f *FastTracer) testCU() {
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CU169)
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CU9929)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CU169)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CU9929)
	f.tracert(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CU169)
	f.tracert(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CU9929)
	f.tracert(TestIPsCollection.Hangzhou.Location, TestIPsCollection.Hangzhou.CU169)

}

func (f *FastTracer) testCM() {
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CM)
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CMIN2)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CM)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CMIN2)
	f.tracert(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CM)
	f.tracert(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CMIN2)
	f.tracert(TestIPsCollection.Hangzhou.Location, TestIPsCollection.Hangzhou.CM)
}

func (f *FastTracer) testEDU() {
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.EDU)
	f.tracert(TestIPsCollection.Hangzhou.Location, TestIPsCollection.Hangzhou.EDU)
	f.tracert(TestIPsCollection.Hefei.Location, TestIPsCollection.Hefei.EDU)
	f.tracert(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.EDU)
	// 科技网暂时算在EDU里面，等拿到了足够多的数据再分离出去，单独用于测试
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CST)
	f.tracert(TestIPsCollection.Hefei.Location, TestIPsCollection.Hefei.CST)
}

func (f *FastTracer) testFast() {
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CT163)
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CU169)
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CM)
	//f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
	//f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CST)
}
