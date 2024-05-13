package tracelog

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/nxtrace/NTrace-core/trace"
)

func RealtimePrinter(res *trace.Result, ttl int) {
	f, err := os.OpenFile("/tmp/trace.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm)
	if err != nil {
		return
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(f)

	multiWriter := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multiWriter)
	log.SetFlags(0)
	var resStr string
	resStr += fmt.Sprintf("%-2d  ", ttl+1)

	// 去重
	var latestIP string
	tmpMap := make(map[string][]string)
	for i, v := range res.Hops[ttl] {
		if v.Address == nil && latestIP != "" {
			tmpMap[latestIP] = append(tmpMap[latestIP], fmt.Sprintf("%s ms", "*"))
			continue
		} else if v.Address == nil {
			continue
		}

		if _, exist := tmpMap[v.Address.String()]; !exist {
			tmpMap[v.Address.String()] = append(tmpMap[v.Address.String()], strconv.Itoa(i))
			// 首次进入
			if latestIP == "" {
				for j := 0; j < i; j++ {
					tmpMap[v.Address.String()] = append(tmpMap[v.Address.String()], fmt.Sprintf("%s ms", "*"))
				}
			}
			latestIP = v.Address.String()
		}

		tmpMap[v.Address.String()] = append(tmpMap[v.Address.String()], fmt.Sprintf("%.2f ms", v.RTT.Seconds()*1000))
	}

	if latestIP == "" {
		resStr += fmt.Sprintf("%s\n", "*")
		log.Print(resStr)
		return
	}

	var blockDisplay = false
	for ip, v := range tmpMap {
		if blockDisplay {
			resStr += fmt.Sprintf("%4s", "")
		}
		if net.ParseIP(ip).To4() == nil {
			resStr += fmt.Sprintf("%-25s ", ip)
		} else {
			resStr += fmt.Sprintf("%-15s ", ip)
		}

		i, _ := strconv.Atoi(v[0])

		if res.Hops[ttl][i].Geo.Asnumber != "" {
			resStr += fmt.Sprintf("AS%-7s", res.Hops[ttl][i].Geo.Asnumber)
		} else {
			resStr += fmt.Sprintf(" %-8s", "*")
		}

		if net.ParseIP(ip).To4() != nil {
			whoisFormat := strings.Split(res.Hops[ttl][i].Geo.Whois, "-")
			if len(whoisFormat) > 1 {
				whoisFormat[0] = strings.Join(whoisFormat[:2], "-")
			}

			if whoisFormat[0] != "" {
				whoisFormat[0] = "[" + whoisFormat[0] + "]"
			}
			resStr += fmt.Sprintf("%-16s", whoisFormat[0])
		}

		if res.Hops[ttl][i].Geo.Country == "" {
			res.Hops[ttl][i].Geo.Country = "LAN Address"
		}

		if net.ParseIP(ip).To4() != nil {

			resStr += fmt.Sprintf(" %s %s %s %s %-6s\n    %-39s   ", res.Hops[ttl][i].Geo.Country, res.Hops[ttl][i].Geo.Prov, res.Hops[ttl][i].Geo.City, res.Hops[ttl][i].Geo.District, res.Hops[ttl][i].Geo.Owner, res.Hops[ttl][i].Hostname)
		} else {
			resStr += fmt.Sprintf(" %s %s %s %s %-6s\n    %-35s ", res.Hops[ttl][i].Geo.Country, res.Hops[ttl][i].Geo.Prov, res.Hops[ttl][i].Geo.City, res.Hops[ttl][i].Geo.District, res.Hops[ttl][i].Geo.Owner, res.Hops[ttl][i].Hostname)
		}

		for j := 1; j < len(v); j++ {
			if len(v) == 2 || j == 1 {
				resStr += v[j]
			} else {
				resStr += fmt.Sprintf("/ %s", v[j])
			}
		}
		log.Print(resStr)
		blockDisplay = true
	}
}
