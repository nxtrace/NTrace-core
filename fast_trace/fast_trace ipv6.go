package fastTrace

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/tracelog"
	"github.com/nxtrace/NTrace-core/util"
	"github.com/nxtrace/NTrace-core/wshandle"
	"log"
	"os"
	"os/signal"
	"strings"
)

//var pFastTracer ParamsFastTrace

func (f *FastTracer) tracert_v6(location string, ispCollection ISPCollection) {
	fmt.Fprintf(color.Output, "%s\n", color.New(color.FgYellow, color.Bold).Sprintf("『%s %s 』", location, ispCollection.ISPName))
	fmt.Printf("traceroute to %s, %d hops max, %d byte packets, %s mode\n", ispCollection.IPv6, f.ParamsFastTrace.MaxHops, f.ParamsFastTrace.PktSize, strings.ToUpper(string(f.TracerouteMethod)))

	// ip, err := util.DomainLookUp(ispCollection.IPv6, "6", "", true)
	ip, err := util.DomainLookUp(ispCollection.IPv6, "6", f.ParamsFastTrace.Dot, true)
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
		log.Printf("traceroute to %s, %d hops max, %d byte packets, %s mode\n", ispCollection.IPv6, f.ParamsFastTrace.MaxHops, f.ParamsFastTrace.PktSize, strings.ToUpper(string(f.TracerouteMethod)))
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

func (f *FastTracer) testAll_v6() {
	f.testCT_v6()
	println()
	f.testCU_v6()
	println()
	f.testCM_v6()
	println()
	f.testEDU_v6()
}

func (f *FastTracer) testCT_v6() {
	f.tracert_v6(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CT163)
	f.tracert_v6(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CT163)
	f.tracert_v6(TestIPsCollection.Hangzhou.Location, TestIPsCollection.Hangzhou.CT163)
	f.tracert_v6(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CT163)
}

func (f *FastTracer) testCU_v6() {
	f.tracert_v6(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CU169)
	f.tracert_v6(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CU169)
	f.tracert_v6(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CU9929)
	f.tracert_v6(TestIPsCollection.Hangzhou.Location, TestIPsCollection.Hangzhou.CU169)
	f.tracert_v6(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CU169)
}

func (f *FastTracer) testCM_v6() {
	f.tracert_v6(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CM)
	f.tracert_v6(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CM)
	f.tracert_v6(TestIPsCollection.Hangzhou.Location, TestIPsCollection.Hangzhou.CM)
	f.tracert_v6(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CM)
}

func (f *FastTracer) testEDU_v6() {
	f.tracert_v6(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
	f.tracert_v6(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.EDU)
	f.tracert_v6(TestIPsCollection.Hangzhou.Location, TestIPsCollection.Hangzhou.EDU)
	f.tracert_v6(TestIPsCollection.Hefei.Location, TestIPsCollection.Hefei.EDU)
	f.tracert_v6(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.EDU)
	// 科技网暂时算在EDU里面，等拿到了足够多的数据再分离出去，单独用于测试
	f.tracert_v6(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CST)
}

func (f *FastTracer) testFast_v6() {
	f.tracert_v6(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CT163)
	f.tracert_v6(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CU169)
	f.tracert_v6(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CM)
	//f.tracert_v6(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
	//f.tracert_v6(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CST)
}

func FastTestv6(traceMode trace.Method, outEnable bool, paramsFastTrace ParamsFastTrace) {
	var c string

	oe = outEnable

	fmt.Println("您想测试哪些ISP的路由？\n1. 北京三网快速测试\n2. 全国电信\n3. 全国联通\n4. 全国移动\n5. 全国教育网\n6. 全国五网")
	fmt.Print("请选择选项：")
	_, err := fmt.Scanln(&c)
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
		fmt.Println("[Info] IPv6 UDP Traceroute is not supported right now.")
		os.Exit(0)
	}

	switch c {
	case "1":
		ft.testFast_v6()
	case "2":
		ft.testCT_v6()
	case "3":
		ft.testCU_v6()
	case "4":
		ft.testCM_v6()
	case "5":
		ft.testEDU_v6()
	case "6":
		ft.testAll_v6()
	default:
		ft.testFast_v6()
	}
}
