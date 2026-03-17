package fastTrace

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/tracelog"
	"github.com/nxtrace/NTrace-core/util"
	"github.com/nxtrace/NTrace-core/wshandle"
)

type FastTracer struct {
	TracerouteMethod trace.Method
	ParamsFastTrace  ParamsFastTrace
}

type ParamsFastTrace struct {
	OSType         int
	ICMPMode       int
	SrcDev         string
	SrcAddr        string
	DstPort        int
	BeginHop       int
	MaxHops        int
	MaxAttempts    int
	RDNS           bool
	AlwaysWaitRDNS bool
	Lang           string
	PktSize        int
	TOS            int
	Timeout        time.Duration
	File           string
	Dot            string
}

type IpListElement struct {
	Ip       string
	Desc     string
	Version4 bool // true for IPv4, false for IPv6
}

var oe = false

func resolveTraceMethod(traceMode trace.Method) trace.Method {
	switch traceMode {
	case trace.TCPTrace:
		return trace.TCPTrace
	case trace.UDPTrace:
		return trace.UDPTrace
	default:
		return trace.ICMPTrace
	}
}

func resolveFastTraceSourceAddr(srcDev string, wantV4 bool) string {
	dev, devErr := net.InterfaceByName(srcDev)
	if devErr != nil || dev == nil {
		return ""
	}
	addrs, err := dev.Addrs()
	if err != nil {
		return ""
	}
	var candidate string
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		isV4 := ipNet.IP.To4() != nil
		if isV4 != wantV4 {
			continue
		}
		candidate = ipNet.IP.String()
		parsed := net.ParseIP(candidate)
		if parsed != nil && !(parsed.IsPrivate() || parsed.IsLoopback() ||
			parsed.IsLinkLocalUnicast() || parsed.IsLinkLocalMulticast()) {
			return candidate
		}
	}
	return candidate
}

func withFastTraceSourceAddr(params ParamsFastTrace, wantV4 bool) ParamsFastTrace {
	if params.SrcDev != "" {
		if srcAddr := resolveFastTraceSourceAddr(params.SrcDev, wantV4); srcAddr != "" {
			params.SrcAddr = srcAddr
		}
	}
	return params
}

func promptFastTraceChoice(prompt, defaultChoice string) string {
	fmt.Print(prompt)
	var choice string
	if _, err := fmt.Scanln(&choice); err != nil {
		return defaultChoice
	}
	return choice
}

func initFastTraceWS() *wshandle.WsConn {
	w := wshandle.New()
	w.Interrupt = make(chan os.Signal, 1)
	signal.Notify(w.Interrupt, os.Interrupt)
	return w
}

func closeFastTraceWS(w *wshandle.WsConn) {
	if w != nil {
		w.Close()
	}
}

func newFastTracer(traceMode trace.Method, params ParamsFastTrace) FastTracer {
	return FastTracer{
		TracerouteMethod: resolveTraceMethod(traceMode),
		ParamsFastTrace:  params,
	}
}

func runFastTraceByChoice(ft FastTracer, choice string) {
	switch choice {
	case "2":
		ft.testFastSH()
	case "3":
		ft.testFastGZ()
	case "4":
		ft.testCT()
	case "5":
		ft.testCU()
	case "6":
		ft.testCM()
	case "7":
		ft.testEDU()
	case "8":
		ft.testAll()
	default:
		ft.testFastBJ()
	}
}

func parseIPListLine(line string) (IpListElement, bool) {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 0 {
		return IpListElement{}, false
	}

	ip := parts[0]
	desc := ip
	if len(parts) == 2 {
		desc = parts[1]
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		netIP, err := util.DomainLookUp(ip, "all", "", true)
		if err != nil {
			fmt.Printf("Ignoring invalid IP: %s\n", ip)
			return IpListElement{}, false
		}
		ip = netIP.String()
	}

	return IpListElement{
		Ip:       ip,
		Desc:     desc,
		Version4: strings.Contains(ip, "."),
	}, true
}

func loadIPList(filePath string) []IpListElement {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return nil
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(file)

	ipList := make([]IpListElement, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if ipElem, ok := parseIPListLine(line); ok {
			ipList = append(ipList, ipElem)
		} else if strings.TrimSpace(line) != "" {
			fmt.Printf("Ignoring invalid line: %s\n", line)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading file:", err)
	}
	return ipList
}

func printFileTraceHeader(ip IpListElement, params ParamsFastTrace, tracerouteMethod trace.Method) {
	fmt.Fprintf(color.Output, "%s\n", color.New(color.FgYellow, color.Bold).Sprint("『 "+ip.Desc+"』"))
	dst := ip.Ip
	if util.EnableHidDstIP {
		dst = util.HideIPPart(ip.Ip)
	}
	fmt.Printf("traceroute to %s, %d hops max, %s, %s mode\n", dst, params.MaxHops, trace.FormatPacketSizeLabel(params.PktSize), strings.ToUpper(string(tracerouteMethod)))
}

func buildFileTraceConfig(params ParamsFastTrace, tracerouteMethod trace.Method, ip IpListElement) trace.Config {
	dstIP := net.ParseIP(ip.Ip)
	packetSizeSpec, err := trace.NormalizePacketSize(tracerouteMethod, dstIP, params.PktSize)
	if err != nil {
		log.Fatal(err)
	}
	return trace.Config{
		OSType:           params.OSType,
		ICMPMode:         params.ICMPMode,
		BeginHop:         params.BeginHop,
		DstIP:            dstIP,
		DstPort:          params.DstPort,
		MaxHops:          params.MaxHops,
		NumMeasurements:  3,
		ParallelRequests: 18,
		RDNS:             params.RDNS,
		AlwaysWaitRDNS:   params.AlwaysWaitRDNS,
		PacketInterval:   100,
		TTLInterval:      500,
		IPGeoSource:      ipgeo.GetSource("LeoMoeAPI"),
		Timeout:          params.Timeout,
		SrcAddr:          resolveFastTraceSourceAddr(params.SrcDev, ip.Version4),
		PktSize:          packetSizeSpec.PayloadSize,
		RandomPacketSize: packetSizeSpec.Random,
		TOS:              params.TOS,
		Lang:             params.Lang,
	}
}

func configureFastTraceRealtimePrinter(conf *trace.Config, header string) error {
	if !oe {
		conf.RealtimePrinter = printer.RealtimePrinter
		return nil
	}

	fp, err := os.OpenFile("/tmp/trace.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	log.SetOutput(fp)
	log.SetFlags(0)
	log.Print(header)
	conf.RealtimePrinter = tracelog.RealtimePrinter
	if err := fp.Close(); err != nil {
		log.Fatal(err)
	}
	return nil
}

func runFileTraceTarget(params ParamsFastTrace, tracerouteMethod trace.Method, ip IpListElement) {
	printFileTraceHeader(ip, params, tracerouteMethod)

	conf := buildFileTraceConfig(params, tracerouteMethod, ip)
	header := fmt.Sprintf("『%s』\ntraceroute to %s, %d hops max, %s, %s mode\n", ip.Desc, ip.Ip, params.MaxHops, trace.FormatPacketSizeLabel(params.PktSize), strings.ToUpper(string(tracerouteMethod)))
	if err := configureFastTraceRealtimePrinter(&conf, header); err != nil {
		return
	}

	if _, err := trace.Traceroute(tracerouteMethod, conf); err != nil {
		log.Fatalln(err)
	}
	fmt.Println()
}

func (f *FastTracer) tracert(location string, ispCollection ISPCollection) {
	fmt.Fprintf(color.Output, "%s\n", color.New(color.FgYellow, color.Bold).Sprintf("『%s %s 』", location, ispCollection.ISPName))
	fmt.Printf("traceroute to %s, %d hops max, %s, %s mode\n", ispCollection.IP, f.ParamsFastTrace.MaxHops, trace.FormatPacketSizeLabel(f.ParamsFastTrace.PktSize), strings.ToUpper(string(f.TracerouteMethod)))

	// ip, err := util.DomainLookUp(ispCollection.IP, "4", "", true)
	ip, err := util.DomainLookUp(ispCollection.IP, "4", f.ParamsFastTrace.Dot, true)
	if err != nil {
		log.Fatal(err)
	}
	packetSizeSpec, err := trace.NormalizePacketSize(f.TracerouteMethod, ip, f.ParamsFastTrace.PktSize)
	if err != nil {
		log.Fatal(err)
	}
	var conf = trace.Config{
		OSType:           f.ParamsFastTrace.OSType,
		ICMPMode:         f.ParamsFastTrace.ICMPMode,
		BeginHop:         f.ParamsFastTrace.BeginHop,
		DstIP:            ip,
		DstPort:          f.ParamsFastTrace.DstPort,
		MaxHops:          f.ParamsFastTrace.MaxHops,
		NumMeasurements:  3,
		MaxAttempts:      f.ParamsFastTrace.MaxAttempts,
		ParallelRequests: 18,
		RDNS:             f.ParamsFastTrace.RDNS,
		AlwaysWaitRDNS:   f.ParamsFastTrace.AlwaysWaitRDNS,
		PacketInterval:   100,
		TTLInterval:      500,
		IPGeoSource:      ipgeo.GetSource("LeoMoeAPI"),
		Timeout:          f.ParamsFastTrace.Timeout,
		SrcAddr:          f.ParamsFastTrace.SrcAddr,
		PktSize:          packetSizeSpec.PayloadSize,
		RandomPacketSize: packetSizeSpec.Random,
		TOS:              f.ParamsFastTrace.TOS,
		Lang:             f.ParamsFastTrace.Lang,
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
		log.Printf("traceroute to %s, %d hops max, %s, %s mode\n", ispCollection.IP, f.ParamsFastTrace.MaxHops, trace.FormatPacketSizeLabel(f.ParamsFastTrace.PktSize), strings.ToUpper(string(f.TracerouteMethod)))
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
	oe = outEnable

	if paramsFastTrace.File != "" {
		testFile(paramsFastTrace, traceMode)
		return
	}

	fmt.Println("Hi，欢迎使用 Fast Trace 功能，请注意 Fast Trace 功能只适合新手使用\n因为国内网络复杂，我们设置的测试目标有限，建议普通用户自测以获得更加精准的路由情况")
	fmt.Println("请您选择要测试的 IP 类型\n1. IPv4\n2. IPv6")
	if promptFastTraceChoice("请选择选项：", "1") == "2" {
		paramsFastTrace = withFastTraceSourceAddr(paramsFastTrace, false)
		FastTestv6(traceMode, outEnable, paramsFastTrace)
		return
	}
	paramsFastTrace = withFastTraceSourceAddr(paramsFastTrace, true)

	fmt.Println("您想测试哪些ISP的路由？\n1. 北京三网快速测试\n2. 上海三网快速测试\n3. 广州三网快速测试\n4. 全国电信\n5. 全国联通\n6. 全国移动\n7. 全国教育网\n8. 全国五网")
	choice := promptFastTraceChoice("请选择选项：", "1")

	w := initFastTraceWS()
	defer closeFastTraceWS(w)

	runFastTraceByChoice(newFastTracer(traceMode, paramsFastTrace), choice)
}

func testFile(paramsFastTrace ParamsFastTrace, traceMode trace.Method) {
	w := initFastTraceWS()
	defer closeFastTraceWS(w)

	tracerouteMethod := resolveTraceMethod(traceMode)
	for _, ip := range loadIPList(paramsFastTrace.File) {
		runFileTraceTarget(paramsFastTrace, tracerouteMethod, ip)
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

func (f *FastTracer) testFastBJ() {
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CT163)
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CU169)
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CM)
	//f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
	//f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CST)
}

func (f *FastTracer) testFastSH() {
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CT163)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CU169)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CM)
}

func (f *FastTracer) testFastGZ() {
	f.tracert(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CT163)
	f.tracert(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CU169)
	f.tracert(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CM)
}
