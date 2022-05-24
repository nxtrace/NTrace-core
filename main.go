package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/xgadget-lab/nexttrace/ipgeo"
	"github.com/xgadget-lab/nexttrace/printer"
	"github.com/xgadget-lab/nexttrace/reporter"
	"github.com/xgadget-lab/nexttrace/trace"
	"github.com/xgadget-lab/nexttrace/util"
)

var tcpSYNFlag = flag.Bool("T", false, "Use TCP SYN for tracerouting (default port is 80)")
var udpPackageFlag = flag.Bool("U", false, "Use UDP Package for tracerouting (default port is 53 in UDP)")
var port = flag.Int("p", 80, "Set SYN Traceroute Port")
var numMeasurements = flag.Int("q", 3, "Set the number of probes per each hop.")
var parallelRequests = flag.Int("r", 18, "Set ParallelRequests number. It should be 1 when there is a multi-routing.")
var maxHops = flag.Int("m", 30, "Set the max number of hops (max TTL to be reached).")
var dataOrigin = flag.String("d", "LeoMoeAPI", "Choose IP Geograph Data Provider [LeoMoeAPI, IP.SB, IPInfo, IPInsight]")
var rdnsenable = flag.Bool("rdns", false, "Set whether rDNS will be display")
var routePath = flag.Bool("report", false, "Route Path")
var realtimePrint = flag.Bool("realtime", false, "Output trace results in runtime")
var tablePrint = flag.Bool("table", false, "Output trace results as table")
var version = flag.Bool("V", false, "Check Version")

func flagApply() string {
	flag.Parse()
	if *version {
		printer.Version()
		os.Exit(0)
	}
	ipArg := flag.Args()
	if flag.NArg() != 1 {
		fmt.Println("Args Error\nUsage : ./nexttrace [-T] [-rdns] [-displayMode <displayMode>] [-d <dataOrigin> ] [ -m <hops> ] [ -p <port> ] [ -q <probes> ] [ -r <parallelrequests> ] <hostname>")
		os.Exit(2)
	}
	return ipArg[0]
}

func main() {

	if os.Getuid() != 0 {
		log.Fatalln("Traceroute requires root/sudo privileges.")
	}

	domain := flagApply()

	ip := util.DomainLookUp(domain)
	printer.PrintTraceRouteNav(ip, domain, *dataOrigin)

	var m trace.Method = ""

	switch {
	case *tcpSYNFlag:
		m = trace.TCPTrace
	case *udpPackageFlag:
		m = trace.UDPTrace
	default:
		m = trace.ICMPTrace
	}

	if !*tcpSYNFlag && *port == 80 {
		*port = 53
	}

	var conf = trace.Config{
		DestIP:           ip,
		DestPort:         *port,
		MaxHops:          *maxHops,
		NumMeasurements:  *numMeasurements,
		ParallelRequests: *parallelRequests,
		RDns:             *rdnsenable,
		IPGeoSource:      ipgeo.GetSource(*dataOrigin),
		Timeout:          2 * time.Second,
	}

	if m == trace.ICMPTrace && !*tablePrint {
		conf.RealtimePrinter = printer.RealtimePrinter
	}

	res, err := trace.Traceroute(m, conf)

	if err != nil {
		log.Fatalln(err)
	}

	if *routePath {
		r := reporter.New(res, ip.String())
		r.Print()
		return
	}

	if m == trace.ICMPTrace && *tablePrint {
		printer.TracerouteTablePrinter(res)
	}

	if m == trace.TCPTrace || m == trace.UDPTrace {
		if *realtimePrint {
			printer.TraceroutePrinter(res)
		} else {
			printer.TracerouteTablePrinter(res)
		}
	}

	printer.SpecialThanks()

}
