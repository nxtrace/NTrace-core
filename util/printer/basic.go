package printer

import (
	"fmt"
	"net"
)

func PrintCopyRight() {
	fmt.Println("NextTrace v0.0.6 Alpha \nxgadget-lab 实验室 zhsh (xzhsh.ch) & leo (leo.moe)")
}

func PrintTraceRouteNav(ip net.IP, domain string, dataOrigin string) {
	fmt.Println("IP Geo Data Provider: " + dataOrigin)

	if ip.String() == domain {
		fmt.Printf("traceroute to %s, 30 hops max, 32 byte packets\n", ip.String())
	} else {
		fmt.Printf("traceroute to %s (%s), 30 hops max, 32 byte packets\n", ip.String(), domain)
	}
}
