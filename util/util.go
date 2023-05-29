package util

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/fatih/color"
)

// LocalIPPort get the local ip and port based on our destination ip
func LocalIPPort(dstip net.IP) (net.IP, int) {
	serverAddr, err := net.ResolveUDPAddr("udp", dstip.String()+":12345")
	if err != nil {
		log.Fatal(err)
	}

	// We don't actually connect to anything, but we can determine
	// based on our destination ip what source ip we should use.
	if con, err := net.DialUDP("udp", nil, serverAddr); err == nil {
		defer con.Close()
		if udpaddr, ok := con.LocalAddr().(*net.UDPAddr); ok {
			return udpaddr.IP, udpaddr.Port
		}
	}
	return nil, -1
}

func LocalIPPortv6(dstip net.IP) (net.IP, int) {
	serverAddr, err := net.ResolveUDPAddr("udp", "["+dstip.String()+"]:12345")
	if err != nil {
		log.Fatal(err)
	}

	// We don't actually connect to anything, but we can determine
	// based on our destination ip what source ip we should use.
	if con, err := net.DialUDP("udp", nil, serverAddr); err == nil {
		defer con.Close()
		if udpaddr, ok := con.LocalAddr().(*net.UDPAddr); ok {
			return udpaddr.IP, udpaddr.Port
		}
	}
	return nil, -1
}

func DomainLookUp(host string, ipVersion string, dotServer string, disableOutput bool) net.IP {
	// ipVersion: 4, 6, all
	var (
		r   *net.Resolver
		ips []net.IP
	)

	switch dotServer {
	case "dnssb":
		r = DNSSB()
	case "aliyun":
		r = Aliyun()
	case "dnspod":
		r = Dnspod()
	case "google":
		r = Google()
	case "cloudflare":
		r = Cloudflare()
	default:
		r = newUDPResolver()
	}
	ips_str, err := r.LookupHost(context.Background(), host)
	for _, v := range ips_str {
		ips = append(ips, net.ParseIP(v))
	}
	if err != nil {
		fmt.Println("Domain " + host + " Lookup Fail.")
		os.Exit(1)
	}

	//var ipv6Flag = false
	//TODO: 此处代码暂无意义
	//if ipv6Flag {
	//	fmt.Println("[Info] IPv6 UDP Traceroute is not supported right now.")
	//	if len(ips) == 0 {
	//		os.Exit(0)
	//	}
	//}

	// Filter by IPv4/IPv6
	if ipVersion != "all" {
		var filteredIPs []net.IP
		for _, ip := range ips {
			if ipVersion == "4" && ip.To4() != nil {
				filteredIPs = append(filteredIPs, ip)
			} else if ipVersion == "6" && strings.Contains(ip.String(), ":") {
				filteredIPs = append(filteredIPs, ip)
			}
		}
		ips = filteredIPs
	}

	if (len(ips) == 1) || (disableOutput) {
		return ips[0]
	} else {
		fmt.Println("Please Choose the IP You Want To TraceRoute")
		for i, ip := range ips {
			fmt.Fprintf(color.Output, "%s %s\n",
				color.New(color.FgHiYellow, color.Bold).Sprintf("%d.", i),
				color.New(color.FgWhite, color.Bold).Sprintf("%s", ip),
			)
		}
		var index int
		fmt.Printf("Your Option: ")
		_, err := fmt.Scanln(&index)
		if err != nil {
			index = 0
		}
		if index >= len(ips) || index < 0 {
			fmt.Println("Your Option is invalid")
			os.Exit(3)
		}
		return ips[index]
	}
}

func GetenvDefault(key, defVal string) string {
	val, ok := os.LookupEnv(key)
	if ok {
		return val
	}
	return defVal
}
