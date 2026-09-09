package fastTrace

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
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

var (
	fastTraceTracerouteFn   = trace.Traceroute
	fastTraceDomainLookupFn = util.DomainLookUpWithContext
)

type ParamsFastTrace struct {
	Context         context.Context
	OSType          int
	ICMPMode        int
	SrcDev          string
	SrcAddr         string
	DstPort         int
	BeginHop        int
	MaxHops         int
	MaxAttempts     int
	RDNS            bool
	AlwaysWaitRDNS  bool
	Lang            string
	PktSize         int
	PacketSizeSet   bool
	TOS             int
	Timeout         time.Duration
	File            string
	Dot             string
	OutputPath      string
	NoStopReason    bool
	RuntimePrepared bool
	DataProvider    string
	IPGeoSource     ipgeo.Source
	IPGeoDescriptor func() ipgeo.SourceDescriptor
	DN42            bool
}

type IpListElement struct {
	Ip       string
	Desc     string
	Version4 bool // true for IPv4, false for IPv6
}

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

func fastTraceGeoSource(params ParamsFastTrace) ipgeo.Source {
	if params.IPGeoSource != nil {
		return params.IPGeoSource
	}
	return trace.CachedGeoSource(ipgeo.GetSourceDescriptorWithGeoDNS(fastTraceDataProvider(params), params.Dot))
}

func fastTraceDataProvider(params ParamsFastTrace) string {
	provider := strings.TrimSpace(params.DataProvider)
	if provider == "" && params.DN42 {
		return "DN42"
	}
	if provider == "" {
		return ipgeo.NextTraceAPIProvider
	}
	return provider
}

func fastTraceDN42(params ParamsFastTrace) bool {
	return params.DN42 || strings.EqualFold(strings.TrimSpace(params.DataProvider), "DN42")
}

func pinFastTraceGeoSource(params ParamsFastTrace) ParamsFastTrace {
	if params.IPGeoSource != nil {
		return params
	}
	descriptor := ipgeo.GetSourceDescriptorWithGeoDNS(fastTraceDataProvider(params), params.Dot)
	params.IPGeoSource = trace.CachedGeoSource(descriptor)
	params.IPGeoDescriptor = func() ipgeo.SourceDescriptor { return descriptor }
	return params
}

func isContextStop(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func shouldStopFastTrace(err error) bool {
	if err == nil {
		return false
	}
	if !isContextStop(err) {
		log.Println(err)
	}
	return true
}

func promptFastTraceChoice(ctx context.Context, prompt, defaultChoice string) (string, bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", false
	}

	fmt.Print(prompt)

	choice, err := util.ReadStdinLine(ctx)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", false
	}
	if err != nil {
		if isContextStop(err) {
			return "", false
		}
		return defaultChoice, true
	}
	choice = strings.TrimSpace(choice)
	if choice == "" {
		return defaultChoice, true
	}
	return choice, true
}

func closeFastTraceWS(w *wshandle.WsConn) {
	if w != nil {
		w.Close()
	}
}

var (
	initFastTraceWSFn  = wshandle.NewWithContext
	closeFastTraceWSFn = closeFastTraceWS
)

func openFastTraceWSIfNeeded(params ParamsFastTrace) func() {
	provider := fastTraceDataProvider(params)
	if params.RuntimePrepared ||
		!ipgeo.IsNextTraceAPIProvider(provider) ||
		ipgeo.NextTraceAPIV4TokenConfigured() {
		return func() {}
	}
	w := initFastTraceWSFn(params.Context)
	return func() {
		closeFastTraceWSFn(w)
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

func parseIPListLine(ctx context.Context, line string) (IpListElement, bool) {
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
		netIP, err := util.DomainLookUpWithContext(ctx, ip, "all", "", true)
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

func loadIPList(ctx context.Context, filePath string) []IpListElement {
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
		if ipElem, ok := parseIPListLine(ctx, line); ok {
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

func printFileTraceHeader(ip IpListElement, params ParamsFastTrace, tracerouteMethod trace.Method) error {
	_, titleErr := fmt.Fprintf(color.Output, "%s\n", color.New(color.FgYellow, color.Bold).Sprint("『 "+ip.Desc+"』"))
	dst := ip.Ip
	if util.EnableHidDstIP {
		dst = util.HideIPPart(ip.Ip)
	}
	displayPacketSize := params.PktSize
	if !params.PacketSizeSet {
		displayPacketSize = trace.DefaultPacketSize(tracerouteMethod, net.ParseIP(ip.Ip))
	}
	fmt.Printf("traceroute to %s, %d hops max, %s, %s mode\n", dst, params.MaxHops, trace.FormatPacketSizeLabel(displayPacketSize), strings.ToUpper(string(tracerouteMethod)))
	return titleErr
}

func buildFileTraceConfig(params ParamsFastTrace, tracerouteMethod trace.Method, ip IpListElement) (trace.Config, error) {
	dstIP := net.ParseIP(ip.Ip)
	packetSize := params.PktSize
	if !params.PacketSizeSet {
		packetSize = trace.DefaultPacketSize(tracerouteMethod, dstIP)
	}
	packetSizeSpec, err := trace.NormalizePacketSize(tracerouteMethod, dstIP, packetSize)
	if err != nil {
		return trace.Config{}, err
	}
	return trace.Config{
		Context:          params.Context,
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
		IPGeoSource:      fastTraceGeoSource(params),
		IPGeoDescriptor:  params.IPGeoDescriptor,
		Timeout:          params.Timeout,
		SrcAddr:          params.SrcAddr,
		SourceDevice:     params.SrcDev,
		PktSize:          packetSizeSpec.PayloadSize,
		RandomPacketSize: packetSizeSpec.Random,
		TOS:              params.TOS,
		Lang:             params.Lang,
		DN42:             fastTraceDN42(params),
	}, nil
}

func normalizeFastTraceConfig(method trace.Method, conf trace.Config) (trace.Config, error) {
	return trace.NormalizeExplicitSourceConfig(method, conf)
}

type fastTraceOutputPlan struct {
	noStopReason bool
	file         io.WriteCloser
}

func (plan *fastTraceOutputPlan) printStopReason(reason *trace.StopReason) error {
	if plan != nil && plan.noStopReason {
		return nil
	}
	terminalErr := printer.PrintTraceStopReason(reason)
	var fileErr error
	if plan != nil && plan.file != nil {
		fileErr = printer.WriteTraceStopReason(plan.file, reason)
	}
	return errors.Join(terminalErr, fileErr)
}

func (plan *fastTraceOutputPlan) close() error {
	if plan == nil || plan.file == nil {
		return nil
	}
	return plan.file.Close()
}

func configureFastTraceRealtimePrinter(conf *trace.Config, outputPath, header string, noStopReason bool) (*fastTraceOutputPlan, error) {
	plan := &fastTraceOutputPlan{noStopReason: noStopReason}
	if strings.TrimSpace(outputPath) == "" {
		conf.RealtimePrinter = printer.RealtimePrinter
		return plan, nil
	}

	fp, err := tracelog.OpenFile(outputPath)
	if err != nil {
		log.Printf("fast trace output open failed for %q: %v; falling back to stdout", outputPath, err)
		conf.RealtimePrinter = printer.RealtimePrinter
		return plan, nil
	}
	if err := tracelog.WriteHeader(fp, header); err != nil {
		if closeErr := fp.Close(); closeErr != nil {
			log.Printf("fast trace output close failed for %q: %v", outputPath, closeErr)
		}
		log.Printf("fast trace output header write failed for %q: %v; falling back to stdout", outputPath, err)
		conf.RealtimePrinter = printer.RealtimePrinter
		return plan, nil
	}
	conf.RealtimePrinter = func(res *trace.Result, ttl int) {
		printer.RealtimePrinter(res, ttl)
		if err := tracelog.WriteRealtime(fp, res, ttl); err != nil {
			log.Printf("fast trace output write failed for %q: %v", outputPath, err)
		}
	}
	plan.file = fp
	return plan, nil
}

func runFastTraceOnce(method trace.Method, conf trace.Config, outputPlan *fastTraceOutputPlan) bool {
	res, err := fastTraceTracerouteFn(method, conf)
	if shouldStopFastTrace(err) {
		return false
	}
	if res == nil {
		log.Println("fast trace returned no result")
		return false
	}
	if err := outputPlan.printStopReason(res.StopReason); err != nil {
		log.Printf("fast trace stop reason write failed: %v", err)
	}
	return true
}

func runFileTraceTarget(params ParamsFastTrace, tracerouteMethod trace.Method, ip IpListElement) {
	if err := printFileTraceHeader(ip, params, tracerouteMethod); err != nil {
		log.Printf("fast trace title write failed: %v", err)
	}

	conf, err := buildFileTraceConfig(params, tracerouteMethod, ip)
	if shouldStopFastTrace(err) {
		return
	}
	conf, err = normalizeFastTraceConfig(tracerouteMethod, conf)
	if shouldStopFastTrace(err) {
		return
	}
	displayPacketSize := params.PktSize
	if !params.PacketSizeSet {
		displayPacketSize = trace.DefaultPacketSize(tracerouteMethod, net.ParseIP(ip.Ip))
	}
	header := fmt.Sprintf("『%s』\ntraceroute to %s, %d hops max, %s, %s mode\n", ip.Desc, ip.Ip, params.MaxHops, trace.FormatPacketSizeLabel(displayPacketSize), strings.ToUpper(string(tracerouteMethod)))
	outputPlan, err := configureFastTraceRealtimePrinter(&conf, params.OutputPath, header, params.NoStopReason)
	if err != nil {
		log.Println(err)
		return
	}
	defer func() {
		if closeErr := outputPlan.close(); closeErr != nil {
			log.Println(closeErr)
		}
	}()

	if !runFastTraceOnce(tracerouteMethod, conf, outputPlan) {
		return
	}
	fmt.Println()
}

func (f *FastTracer) tracert(location string, ispCollection ISPCollection) {
	if _, err := fmt.Fprintf(color.Output, "%s\n", color.New(color.FgYellow, color.Bold).Sprintf("『%s %s 』", location, ispCollection.ISPName)); err != nil {
		log.Printf("fast trace title write failed: %v", err)
	}
	displayPacketSize := f.ParamsFastTrace.PktSize
	if !f.ParamsFastTrace.PacketSizeSet {
		displayPacketSize = trace.DefaultPacketSize(f.TracerouteMethod, net.ParseIP(ispCollection.IP))
	}
	fmt.Printf("traceroute to %s, %d hops max, %s, %s mode\n", ispCollection.IP, f.ParamsFastTrace.MaxHops, trace.FormatPacketSizeLabel(displayPacketSize), strings.ToUpper(string(f.TracerouteMethod)))

	// ip, err := util.DomainLookUp(ispCollection.IP, "4", "", true)
	ip, err := fastTraceDomainLookupFn(f.ParamsFastTrace.Context, ispCollection.IP, "4", f.ParamsFastTrace.Dot, true)
	if shouldStopFastTrace(err) {
		return
	}
	packetSize := f.ParamsFastTrace.PktSize
	if !f.ParamsFastTrace.PacketSizeSet {
		packetSize = trace.DefaultPacketSize(f.TracerouteMethod, ip)
	}
	packetSizeSpec, err := trace.NormalizePacketSize(f.TracerouteMethod, ip, packetSize)
	if shouldStopFastTrace(err) {
		return
	}
	var conf = trace.Config{
		Context:          f.ParamsFastTrace.Context,
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
		IPGeoSource:      fastTraceGeoSource(f.ParamsFastTrace),
		IPGeoDescriptor:  f.ParamsFastTrace.IPGeoDescriptor,
		Timeout:          f.ParamsFastTrace.Timeout,
		SrcAddr:          f.ParamsFastTrace.SrcAddr,
		SourceDevice:     f.ParamsFastTrace.SrcDev,
		PktSize:          packetSizeSpec.PayloadSize,
		RandomPacketSize: packetSizeSpec.Random,
		TOS:              f.ParamsFastTrace.TOS,
		Lang:             f.ParamsFastTrace.Lang,
		DN42:             fastTraceDN42(f.ParamsFastTrace),
	}
	conf, err = normalizeFastTraceConfig(f.TracerouteMethod, conf)
	if shouldStopFastTrace(err) {
		return
	}

	header := fmt.Sprintf("『%s %s 』\ntraceroute to %s, %d hops max, %s, %s mode\n",
		location, ispCollection.ISPName, ispCollection.IP, f.ParamsFastTrace.MaxHops, trace.FormatPacketSizeLabel(displayPacketSize), strings.ToUpper(string(f.TracerouteMethod)))
	outputPlan, err := configureFastTraceRealtimePrinter(&conf, f.ParamsFastTrace.OutputPath, header, f.ParamsFastTrace.NoStopReason)
	if err != nil {
		log.Println(err)
		return
	}
	defer func() {
		if closeErr := outputPlan.close(); closeErr != nil {
			log.Println(closeErr)
		}
	}()

	if !runFastTraceOnce(f.TracerouteMethod, conf, outputPlan) {
		return
	}
	fmt.Println()
}

func FastTest(traceMode trace.Method, paramsFastTrace ParamsFastTrace) {
	paramsFastTrace = pinFastTraceGeoSource(paramsFastTrace)
	if paramsFastTrace.File != "" {
		testFile(paramsFastTrace, traceMode)
		return
	}

	fmt.Println("Hi，欢迎使用 Fast Trace 功能，请注意 Fast Trace 功能只适合新手使用\n因为国内网络复杂，我们设置的测试目标有限，建议普通用户自测以获得更加精准的路由情况")
	fmt.Println("请您选择要测试的 IP 类型\n1. IPv4\n2. IPv6")
	ipChoice, ok := promptFastTraceChoice(paramsFastTrace.Context, "请选择选项：", "1")
	if !ok {
		return
	}
	if ipChoice == "2" {
		FastTestv6(traceMode, paramsFastTrace)
		return
	}

	fmt.Println("您想测试哪些ISP的路由？\n1. 北京三网快速测试\n2. 上海三网快速测试\n3. 广州三网快速测试\n4. 全国电信\n5. 全国联通\n6. 全国移动\n7. 全国教育网\n8. 全国五网")
	choice, ok := promptFastTraceChoice(paramsFastTrace.Context, "请选择选项：", "1")
	if !ok {
		return
	}

	cleanupWS := openFastTraceWSIfNeeded(paramsFastTrace)
	defer cleanupWS()

	runFastTraceByChoice(newFastTracer(traceMode, paramsFastTrace), choice)
}

func testFile(paramsFastTrace ParamsFastTrace, traceMode trace.Method) {
	paramsFastTrace = pinFastTraceGeoSource(paramsFastTrace)
	cleanupWS := openFastTraceWSIfNeeded(paramsFastTrace)
	defer cleanupWS()

	tracerouteMethod := resolveTraceMethod(traceMode)
	for _, ip := range loadIPList(paramsFastTrace.Context, paramsFastTrace.File) {
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
