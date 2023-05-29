package fastTrace

import (
	"fmt"
	"github.com/xgadget-lab/nexttrace/util"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/xgadget-lab/nexttrace/ipgeo"
	"github.com/xgadget-lab/nexttrace/printer"
	"github.com/xgadget-lab/nexttrace/trace"
	"github.com/xgadget-lab/nexttrace/tracelog"
	"github.com/xgadget-lab/nexttrace/wshandle"
)

type FastTracer struct {
	TracerouteMethod trace.Method
}

var oe = false

func (f *FastTracer) tracert(location string, ispCollection ISPCollection) {
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
	fmt.Printf("%s『%s %s 』%s\n", printer.YELLOW_PREFIX, location, ispCollection.ISPName, printer.RESET_PREFIX)
	log.Printf("『%s %s 』\n", location, ispCollection.ISPName)
	fmt.Printf("traceroute to %s, 30 hops max, 32 byte packets\n", ispCollection.IP)
	log.Printf("traceroute to %s, 30 hops max, 32 byte packets\n", ispCollection.IP)
	ip := util.DomainLookUp(ispCollection.IP, "4", "", true)
	var conf = trace.Config{
		BeginHop:         1,
		DestIP:           ip,
		DestPort:         80,
		MaxHops:          30,
		NumMeasurements:  3,
		ParallelRequests: 18,
		RDns:             true,
		PacketInterval:   100,
		TTLInterval:      500,
		IPGeoSource:      ipgeo.GetSource("LeoMoeAPI"),
		Timeout:          1 * time.Second,
	}

	if oe {
		conf.RealtimePrinter = tracelog.RealtimePrinter
	} else {
		conf.RealtimePrinter = printer.RealtimePrinter
	}

	_, err = trace.Traceroute(f.TracerouteMethod, conf)

	if err != nil {
		log.Fatal(err)
	}
	println()
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
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CT163)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CTCN2)
	f.tracert(TestIPsCollection.Hangzhou.Location, TestIPsCollection.Hangzhou.CT163)
	f.tracert(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CT163)
}

func (f *FastTracer) testCU() {
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CU169)
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CU9929)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CU169)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CU9929)
	f.tracert(TestIPsCollection.Hangzhou.Location, TestIPsCollection.Hangzhou.CU169)
	f.tracert(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CU169)
}

func (f *FastTracer) testCM() {
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CM)
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.CMIN2)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CM)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.CMIN2)
	f.tracert(TestIPsCollection.Hangzhou.Location, TestIPsCollection.Hangzhou.CM)
	f.tracert(TestIPsCollection.Guangzhou.Location, TestIPsCollection.Guangzhou.CM)
}

func (f *FastTracer) testEDU() {
	f.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
	f.tracert(TestIPsCollection.Shanghai.Location, TestIPsCollection.Shanghai.EDU)
	f.tracert(TestIPsCollection.Hangzhou.Location, TestIPsCollection.Hangzhou.EDU)
	// 科技网暂时算在EDU里面，等拿到了足够多的数据再分离出去，单独用于测试
	f.tracert(TestIPsCollection.Hefei.Location, TestIPsCollection.Hefei.CST)
}

func FastTest(tm bool, outEnable bool) {
	var c string

	oe = outEnable
	fmt.Println("Hi，欢迎使用 Fast Trace 功能，请注意 Fast Trace 功能只适合新手使用\n因为国内网络复杂，我们设置的测试目标有限，建议普通用户自测以获得更加精准的路由情况")
	fmt.Println("请您选择要测试的 IP 类型\n1. IPv4\n2. IPv6")
	fmt.Print("请选择选项：")
	_, err := fmt.Scanln(&c)
	if err != nil {
		c = "1"
	}
	if c == "2" {
		FastTestv6(tm, outEnable)
		return
	}

	fmt.Println("您想测试哪些ISP的路由？\n1. 国内四网\n2. 电信\n3. 联通\n4. 移动\n5. 教育网")
	fmt.Print("请选择选项：")
	_, err = fmt.Scanln(&c)
	if err != nil {
		c = "1"
	}

	ft := FastTracer{}

	// 建立 WebSocket 连接
	w := wshandle.New()
	w.Interrupt = make(chan os.Signal, 1)
	signal.Notify(w.Interrupt, os.Interrupt)
	defer func() {
		w.Conn.Close()
	}()

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
