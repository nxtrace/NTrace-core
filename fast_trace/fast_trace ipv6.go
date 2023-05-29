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

func (f *FastTracer) tracert_v6(location string, ispCollection ISPCollection) {
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
	fmt.Printf("traceroute to %s, 30 hops max, 32 byte packets\n", ispCollection.IPv6)
	log.Printf("traceroute to %s, 30 hops max, 32 byte packets\n", ispCollection.IPv6)
	ip := util.DomainLookUp(ispCollection.IP, "6", "", true)
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
}

func FastTestv6(tm bool, outEnable bool) {
	var c string

	oe = outEnable

	fmt.Println("您想测试哪些ISP的路由？\n1. 国内四网\n2. 电信\n3. 联通\n4. 移动\n5. 教育网")
	fmt.Print("请选择选项：")
	_, err := fmt.Scanln(&c)
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
		ft.testAll_v6()
	case "2":
		ft.testCT_v6()
	case "3":
		ft.testCU_v6()
	case "4":
		ft.testCM_v6()
	case "5":
		ft.testEDU_v6()
	default:
		ft.testAll_v6()
	}
}
