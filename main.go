package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/xgadget-lab/nexttrace/config"
	fastTrace "github.com/xgadget-lab/nexttrace/fast_trace"
	"github.com/xgadget-lab/nexttrace/ipgeo"
	"github.com/xgadget-lab/nexttrace/printer"
	"github.com/xgadget-lab/nexttrace/reporter"
	"github.com/xgadget-lab/nexttrace/trace"
	"github.com/xgadget-lab/nexttrace/util"
)

var fSet = flag.NewFlagSet("", flag.ExitOnError)
var fastTest = fSet.Bool("f", false, "One-Key Fast Traceroute")
var tcpSYNFlag = fSet.Bool("T", false, "Use TCP SYN for tracerouting (default port is 80)")
var udpPackageFlag = fSet.Bool("U", false, "Use UDP Package for tracerouting (default port is 53 in UDP)")
var port = fSet.Int("p", 80, "Set SYN Traceroute Port")
var manualConfig = fSet.Bool("c", false, "Manual Config [Advanced]")
var numMeasurements = fSet.Int("q", 3, "Set the number of probes per each hop.")
var parallelRequests = fSet.Int("r", 18, "Set ParallelRequests number. It should be 1 when there is a multi-routing.")
var maxHops = fSet.Int("m", 30, "Set the max number of hops (max TTL to be reached).")
var dataOrigin = fSet.String("d", "", "Choose IP Geograph Data Provider [LeoMoeAPI, IP.SB, IPInfo, IPInsight, IPAPI.com]")
var noRdns = fSet.Bool("n", false, "Disable IP Reverse DNS lookup")
var routePath = fSet.Bool("report", false, "Route Path")
var tablePrint = fSet.Bool("table", false, "Output trace results as table")
var ver = fSet.Bool("V", false, "Check Version")

func printArgHelp() {
	fmt.Println("\nArgs Error\nUsage : 'nexttrace [option...] HOSTNAME' or 'nexttrace HOSTNAME [option...]'\nOPTIONS: [-VTU] [-d DATAORIGIN.STR ] [ -m TTL ] [ -p PORT ] [ -q PROBES.COUNT ] [ -r PARALLELREQUESTS.COUNT ] [-rdns] [ -table ] -report")
	fSet.PrintDefaults()
	os.Exit(2)
}

func flagApply() string {
	printer.Version()

	target := ""
	if len(os.Args) < 2 {
		printArgHelp()
	}

	// flag parse
	if !strings.HasPrefix(os.Args[1], "-") {
		target = os.Args[1]
		fSet.Parse(os.Args[2:])
	} else {
		fSet.Parse(os.Args[1:])
		target = fSet.Arg(0)
	}

	// Print Version
	if *ver {
		os.Exit(0)
	}

	// Advanced Config
	if *manualConfig {
		if _, err := config.Generate(); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	// -f Fast Test
	if *fastTest {
		fastTrace.FastTest(*tcpSYNFlag)
		os.Exit(0)
	}

	if target == "" {
		printArgHelp()
	}
	return target
}

func main() {

	domain := flagApply()

	if os.Getuid() != 0 {
		log.Fatalln("Traceroute requires root/sudo privileges.")
	}

	configData, err := config.Read()

	// Initialize Default Config
	if err != nil || configData.DataOrigin == "" {
		if configData, err = config.AutoGenerate(); err != nil {
			log.Fatal(err)
		}
	}

	// Set Token from Config
	ipgeo.SetToken(configData.Token)

	// Check Whether User has specified IP Geograph Data Provider
	if *dataOrigin == "" {
		// Use Default Data Origin with Config
		*dataOrigin = configData.DataOrigin
	}

	var ip net.IP

	if *tcpSYNFlag || *udpPackageFlag {
		ip = util.DomainLookUp(domain, true)
	} else {
		ip = util.DomainLookUp(domain, false)
	}

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
		RDns:             !*noRdns,
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

	if (*tcpSYNFlag && *udpPackageFlag) || *tablePrint || configData.TablePrintDefault {
		printer.TracerouteTablePrinter(res)
	} else if *tcpSYNFlag || *udpPackageFlag {
		printer.TraceroutePrinter(res)
	}

	if *routePath || configData.AlwaysRoutePath {
		r := reporter.New(res, ip.String())
		r.Print()
	}
}
