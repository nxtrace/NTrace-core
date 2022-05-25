package printer

import (
	"fmt"
	"net"
)

var version = "v0.0.0.alpha"
var buildDate = ""
var commitID = ""

func Version() {
	fmt.Println("NextTrace", version, buildDate, commitID)
	fmt.Println("XGadget-lab Leo (leo.moe) & Vincent (vincent.moe) & zhshch (xzhsh.ch)")
}

func PrintTraceRouteNav(ip net.IP, domain string, dataOrigin string) {
	fmt.Println("IP Geo Data Provider: " + dataOrigin)

	if ip.String() == domain {
		fmt.Printf("traceroute to %s, 30 hops max, 32 byte packets\n", ip.String())
	} else {
		fmt.Printf("traceroute to %s (%s), 30 hops max, 32 byte packets\n", ip.String(), domain)
	}
}
