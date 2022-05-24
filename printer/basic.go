package printer

import (
	"fmt"
	"net"
)

func SpecialThanks() {
	fmt.Println("\nSpecial thanks to Vincent Young (OwO Network / vincent.moe) for his help and support for this project.")
}

func Version() {
	fmt.Println("NextTrace v0.1.2-Beta.missuo-special-version")
}

func PrintTraceRouteNav(ip net.IP, domain string, dataOrigin string) {
	fmt.Println("IP Geo Data Provider: " + dataOrigin)

	if ip.String() == domain {
		fmt.Printf("traceroute to %s, 30 hops max, 32 byte packets\n", ip.String())
	} else {
		fmt.Printf("traceroute to %s (%s), 30 hops max, 32 byte packets\n", ip.String(), domain)
	}
}
