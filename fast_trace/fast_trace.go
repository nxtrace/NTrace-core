package fastTrace

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/xgadget-lab/nexttrace/ipgeo"
	"github.com/xgadget-lab/nexttrace/printer"
	"github.com/xgadget-lab/nexttrace/trace"
)

type FastTracer struct {
	TracerouteMethod trace.Method
}

func (f *FastTracer) tracert(location string, ispCollection ISPCollection) {
	fmt.Printf("『%s %s 』\n", location, ispCollection.ISPName)
	fmt.Printf("traceroute to %s, 30 hops max, 32 byte packets\n", ispCollection.IP)
	ip := net.ParseIP(ispCollection.IP)
	var conf = trace.Config{
		BeginHop:         1,
		DestIP:           ip,
		DestPort:         80,
		MaxHops:          30,
		NumMeasurements:  3,
		ParallelRequests: 18,
		RDns:             true,
		IPGeoSource:      ipgeo.GetSource("LeoMoeAPI"),
		Timeout:          1 * time.Second,
	}

	if f.TracerouteMethod == trace.ICMPTrace {
		conf.RealtimePrinter = printer.RealtimePrinter
	}

	res, err := trace.Traceroute(f.TracerouteMethod, conf)

	if err != nil {
		log.Fatal(err)
	}

	if f.TracerouteMethod == trace.TCPTrace {
		printer.TracerouteTablePrinter(res)
	}
}

func (f *FastTracer) testAll() {
	f.testCT()
	f.testCU()
	f.testCM()
	f.testEDU()
}

func (f *FastTracer) testCT() {
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CT163)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CT163)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CTCN2)
	f.tracert(TestIPsCollection.Hangzhou.Location, TestIPsCollection.Hangzhou.CT163)
	f.tracert(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CT163)
}

func (f *FastTracer) testCU() {
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CU169)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CU169)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CU9929)
	f.tracert(TestIPsCollection.Hangzhou.Location, TestIPsCollection.Hangzhou.CU169)
	f.tracert(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CU169)
}

func (f *FastTracer) testCM() {
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CM)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CM)
	f.tracert(TestIPsCollection.Hangzhou.Location, TestIPsCollection.Hangzhou.CM)
	f.tracert(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CM)
}

func (f *FastTracer) testEDU() {
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.EDU)
	f.tracert(TestIPsCollection.Hangzhou.Location, TestIPsCollection.Hangzhou.EDU)
	f.tracert(TestIPsCollection.Hefei.Location, TestIPsCollection.Hefei.EDU)
	// 科技网暂时算在EDU里面，等拿到了足够多的数据再分离出去，单独用于测试
	f.tracert(TestIPsCollection.Hefei.Location, TestIPsCollection.Hefei.CST)
}

func FastTest(tm bool) {
	var c string

	fmt.Println("您想测试哪些ISP的路由？\n1. 国内四网\n2. 电信\n3. 联通\n4. 移动\n5. 教育网")
	fmt.Print("请选择选项：")
	fmt.Scanln(&c)

	ft := FastTracer{}

	if !tm {
		ft.TracerouteMethod = trace.ICMPTrace
		fmt.Println("您将默认使用ICMP协议进行路由跟踪，如果您想使用TCP SYN进行路由跟踪，可以加入 -T 参数")
	} else {
		ft.TracerouteMethod = trace.TCPTrace
	}

	switch c {
	case "1":
		ft.testAll()
	case "2":
		ft.testCT()
	case "3":
		ft.testCU()
	case "4":
		ft.testCM()
	case "5":
		ft.testEDU()
	default:
		ft.testAll()
	}
}
