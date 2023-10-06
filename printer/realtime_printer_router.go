package printer

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/nxtrace/NTrace-core/trace"
)

func RealtimePrinterWithRouter(res *trace.Result, ttl int) {
	fmt.Printf("%s  ", color.New(color.FgHiYellow, color.Bold).Sprintf("%-2d", ttl+1))

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
		fmt.Fprintf(color.Output, "%s\n",
			color.New(color.FgWhite, color.Bold).Sprintf("*"),
		)
		return
	}

	var blockDisplay = false
	for ip, v := range tmpMap {
		if blockDisplay {
			fmt.Printf("%4s", "")
		}
		if net.ParseIP(ip).To4() == nil {
			fmt.Fprintf(color.Output, "%s",
				color.New(color.FgWhite, color.Bold).Sprintf("%-25s", ip),
			)
		} else {
			fmt.Fprintf(color.Output, "%s",
				color.New(color.FgWhite, color.Bold).Sprintf("%-15s", ip),
			)
		}

		i, _ := strconv.Atoi(v[0])

		if res.Hops[ttl][i].Geo.Asnumber != "" {
			fmt.Fprintf(color.Output, " %s", color.New(color.FgHiGreen, color.Bold).Sprintf("AS%-6s", res.Hops[ttl][i].Geo.Asnumber))
		} else {
			fmt.Printf(" %-8s", "*")
		}

		if net.ParseIP(ip).To4() != nil {
			whoisFormat := strings.Split(res.Hops[ttl][i].Geo.Whois, "-")
			if len(whoisFormat) > 1 {
				whoisFormat[0] = strings.Join(whoisFormat[:2], "-")
			}

			if whoisFormat[0] != "" {
				whoisFormat[0] = "[" + whoisFormat[0] + "]"
			}
			fmt.Fprintf(color.Output, " %s", color.New(color.FgHiGreen, color.Bold).Sprintf("%-16s", whoisFormat[0]))
		}

		if res.Hops[ttl][i].Geo.Country == "" {
			res.Hops[ttl][i].Geo.Country = "LAN Address"
		}

		if net.ParseIP(ip).To4() != nil {

			fmt.Fprintf(color.Output, " %s %s %s %s %s\n    %s   ",
				color.New(color.FgWhite, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.Country),
				color.New(color.FgWhite, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.Prov),
				color.New(color.FgWhite, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.City),
				color.New(color.FgWhite, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.District),
				fmt.Sprintf("%-6s", res.Hops[ttl][i].Geo.Owner),
				color.New(color.FgHiBlack, color.Bold).Sprintf("%-39s", res.Hops[ttl][i].Hostname),
			)
		} else {
			fmt.Fprintf(color.Output, " %s %s %s %s %s\n    %s   ",
				color.New(color.FgWhite, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.Country),
				color.New(color.FgWhite, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.Prov),
				color.New(color.FgWhite, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.City),
				color.New(color.FgWhite, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.District),
				fmt.Sprintf("%-6s", res.Hops[ttl][i].Geo.Owner),
				color.New(color.FgHiBlack, color.Bold).Sprintf("%-32s", res.Hops[ttl][i].Hostname),
			)
		}

		for j := 1; j < len(v); j++ {
			if len(v) == 2 || j == 1 {
				fmt.Fprintf(color.Output, "%s",
					color.New(color.FgHiCyan, color.Bold).Sprintf("%s", v[j]),
				)
			} else {
				fmt.Fprintf(color.Output, " / %s",
					color.New(color.FgHiCyan, color.Bold).Sprintf("%s", v[j]),
				)
			}
		}
		i = 0
		fmt.Println()
		if res.Hops[ttl][i].Geo != nil && !blockDisplay {
			fmt.Fprintf(color.Output, "%s   %s %s %s   %s\n",
				color.New(color.FgWhite, color.Bold).Sprintf("-"),
				color.New(color.FgHiYellow, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.Prefix),
				color.New(color.FgWhite, color.Bold).Sprintf("路由表"),
				color.New(color.FgHiCyan, color.Bold).Sprintf("Beta"),
				color.New(color.FgWhite, color.Bold).Sprintf("-"),
			)
			GetRouter(&res.Hops[ttl][i].Geo.Router, "AS"+res.Hops[ttl][i].Geo.Asnumber)
		}
		blockDisplay = true
	}
}

func GetRouter(r *map[string][]string, node string) {
	routeMap := *r
	for _, v := range routeMap[node] {
		if len(routeMap[v]) != 0 {
			fmt.Fprintf(color.Output, "    %s %s %s\n",
				color.New(color.FgWhite, color.Bold).Sprintf("%s", routeMap[v][0]),
				color.New(color.FgWhite, color.Bold).Sprintf("%s", v),
				color.New(color.FgHiBlue, color.Bold).Sprintf("%s", node),
			)
		} else {
			fmt.Fprintf(color.Output, "    %s %s\n",
				color.New(color.FgWhite, color.Bold).Sprintf("%s", v),
				color.New(color.FgHiBlue, color.Bold).Sprintf("%s", node),
			)
		}

	}
}
