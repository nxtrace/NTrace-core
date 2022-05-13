package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/xgadget-lab/nexttrace/methods"
	"github.com/xgadget-lab/nexttrace/methods/tcp"
	"github.com/xgadget-lab/nexttrace/methods/udp"
	"github.com/xgadget-lab/nexttrace/util"
	"github.com/xgadget-lab/nexttrace/util/printer"
)

var tcpSYNFlag = flag.Bool("T", false, "Use TCP SYN for tracerouting (default port is 80 in TCP, 53 in UDP)")
var port = flag.Int("p", 80, "Set SYN Traceroute Port")
var numMeasurements = flag.Int("q", 3, "Set the number of probes per each hop.")
var parallelRequests = flag.Int("r", 18, "Set ParallelRequests number. It should be 1 when there is a multi-routing.")
var maxHops = flag.Int("m", 30, "Set the max number of hops (max TTL to be reached).")
var dataOrigin = flag.String("d", "LeoMoeAPI", "Choose IP Geograph Data Provider [LeoMoeAPI, IP.SB, IPInfo, IPInsight]")
var displayMode = flag.String("displayMode", "table", "Choose The Display Mode [table, classic]")
var rdnsenable = flag.Bool("rdns", false, "Set whether rDNS will be display")

func main() {
	printer.PrintCopyRight()
	domain := flagApply()
	ip := util.DomainLookUp(domain)
	printer.PrintTraceRouteNav(ip, domain, *dataOrigin)

	if *tcpSYNFlag {
		tcpTraceroute := tcp.New(ip, methods.TracerouteConfig{
			MaxHops:          uint16(*maxHops),
			NumMeasurements:  uint16(*numMeasurements),
			ParallelRequests: uint16(*parallelRequests),
			Port:             *port,
			Timeout:          time.Second / 2,
		})
		res, err := tcpTraceroute.Start()

		if err != nil {
			fmt.Println("请赋予 sudo (root) 权限运行本程序")
		} else {
			util.Printer(&util.PrinterConfig{
				IP:          ip,
				DisplayMode: *displayMode,
				DataOrigin:  *dataOrigin,
				Rdnsenable:  *rdnsenable,
				Results:     *res,
			})
		}

	} else {
		if *port == 80 {
			*port = 53
		}
		udpTraceroute := udp.New(ip, true, methods.TracerouteConfig{
			MaxHops:          uint16(*maxHops),
			NumMeasurements:  uint16(*numMeasurements),
			ParallelRequests: uint16(*parallelRequests),
			Port:             *port,
			Timeout:          2 * time.Second,
		})
		res, err := udpTraceroute.Start()

		if err != nil {
			fmt.Println("请赋予 sudo (root) 权限运行本程序")
		} else {
			util.Printer(&util.PrinterConfig{
				IP:          ip,
				DisplayMode: *displayMode,
				DataOrigin:  *dataOrigin,
				Rdnsenable:  *rdnsenable,
				Results:     *res,
			})
		}
	}
}

func flagApply() string {
	flag.Parse()
	ipArg := flag.Args()
	if flag.NArg() != 1 {
		fmt.Println("Args Error\nUsage : ./bettertrace [-T] [-d <dataOrigin> ] [ -m <hops> ] [ -p <port> ] [ -q <probes> ] [ -r <parallelrequests> ] <hostname>")
		os.Exit(2)
	}
	return ipArg[0]
}
