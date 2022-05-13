package printer

import (
	"fmt"
	"net"
	"strings"
	"traceroute/methods"
	"traceroute/util"
	"traceroute/util/geodata"
)

var dataOrigin string

func TraceroutePrinter(ip net.IP, res *map[uint16][]methods.TracerouteHop, dataOrigin string) {
	hopIndex := uint16(1)
	for hopIndex <= 29 {
		for k, v := range *res {
			if k == hopIndex {
				fmt.Print(k)
				for _, v2 := range v {
					ch := make(chan uint16)
					go hopPrinter(hopIndex, ip, v2, ch)
					hopIndex = <-ch
				}
				hopIndex = hopIndex + 1
				break
			}
		}
	}
}

func hopPrinter(hopIndex uint16, ip net.IP, v2 methods.TracerouteHop, c chan uint16) {
	if v2.Address == nil {
		fmt.Println("\t*")
	} else {
		ip_str := fmt.Sprintf("%s", v2.Address)

		ptr, err := net.LookupAddr(ip_str)

		ch_b := make(chan util.IPGeoData)

		if dataOrigin == "LeoMoeAPI" {
			go geodata.GetIPGeo(ip_str, ch_b)

		} else if dataOrigin == "IP.SB" {
			go geodata.GetIPGeoByIPSB(ip_str, ch_b)

		} else if dataOrigin == "IPInfo" {
			go geodata.GetIPGeoByIPInfo(ip_str, ch_b)

		} else if dataOrigin == "IPInsight" {
			go geodata.GetIPGeoByIPInsight(ip_str, ch_b)

		} else {
			go geodata.GetIPGeo(ip_str, ch_b)
		}

		iPGeoData := <-ch_b

		if ip.String() == ip_str {
			hopIndex = 30
			iPGeoData.Owner = iPGeoData.Isp
		}

		if strings.Index(ip_str, "9.31.") == 0 || strings.Index(ip_str, "11.72.") == 0 {
			fmt.Printf("\t%-15s %.2fms * 局域网, 腾讯云\n", v2.Address, v2.RTT.Seconds()*1000)
			c <- hopIndex
			return
		}

		if strings.Index(ip_str, "11.13.") == 0 {
			fmt.Printf("\t%-15s %.2fms * 局域网, 阿里云\n", v2.Address, v2.RTT.Seconds()*1000)
			c <- hopIndex
			return
		}

		if iPGeoData.Owner == "" {
			iPGeoData.Owner = iPGeoData.Isp
		}

		if iPGeoData.Asnumber == "" {
			iPGeoData.Asnumber = "*"
		} else {
			iPGeoData.Asnumber = "AS" + iPGeoData.Asnumber
		}

		if iPGeoData.District != "" {
			iPGeoData.City = iPGeoData.City + ", " + iPGeoData.District
		}

		if iPGeoData.Country == "" {
			fmt.Printf("\t%-15s %.2fms * 局域网\n", v2.Address, v2.RTT.Seconds()*1000)
			c <- hopIndex
			return
		}

		if iPGeoData.Prov != "" && iPGeoData.City == "" {
			// Province Only
			if err != nil {
				fmt.Printf("\t%-15s %.2fms %s %s, %s, %s\n", v2.Address, v2.RTT.Seconds()*1000, iPGeoData.Asnumber, iPGeoData.Country, iPGeoData.Prov, iPGeoData.Owner)
			} else {
				fmt.Printf("\t%-15s (%s) %.2fms %s %s, %s, %s\n", ptr[0], v2.Address, v2.RTT.Seconds()*1000, iPGeoData.Asnumber, iPGeoData.Country, iPGeoData.Prov, iPGeoData.Owner)
			}
		} else if iPGeoData.Prov == "" && iPGeoData.City == "" {

			if err != nil {
				fmt.Printf("\t%-15s %.2fms %s %s, %s, %s 骨干网\n", v2.Address, v2.RTT.Seconds()*1000, iPGeoData.Asnumber, iPGeoData.Country, iPGeoData.Owner, iPGeoData.Owner)
			} else {
				fmt.Printf("\t%-15s (%s) %.2fms %s %s, %s, %s 骨干网\n", ptr[0], v2.Address, v2.RTT.Seconds()*1000, iPGeoData.Asnumber, iPGeoData.Country, iPGeoData.Owner, iPGeoData.Owner)
			}
		} else {

			if err != nil {
				fmt.Printf("\t%-15s %.2fms %s %s, %s, %s, %s\n", v2.Address, v2.RTT.Seconds()*1000, iPGeoData.Asnumber, iPGeoData.Country, iPGeoData.Prov, iPGeoData.City, iPGeoData.Owner)
			} else {
				fmt.Printf("\t%-15s (%s) %.2fms %s %s, %s, %s, %s\n", ptr[0], v2.Address, v2.RTT.Seconds()*1000, iPGeoData.Asnumber, iPGeoData.Country, iPGeoData.Prov, iPGeoData.City, iPGeoData.Owner)
			}
		}
	}
	c <- hopIndex
}
