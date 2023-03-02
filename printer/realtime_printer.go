package printer

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/xgadget-lab/nexttrace/trace"
)

func RealtimePrinter(res *trace.Result, ttl int) {
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
		_, err := fmt.Fprintf(color.Output, "%s\n",
			color.New(color.FgWhite, color.Bold).Sprintf("*"),
		)
		if err != nil {
			return
		}
		return
	}

	var blockDisplay = false
	for ip, v := range tmpMap {
		if blockDisplay {
			fmt.Printf("%4s", "")
		}
		if net.ParseIP(ip).To4() == nil {
			_, err := fmt.Fprintf(color.Output, "%s",
				color.New(color.FgWhite, color.Bold).Sprintf("%-25s", ip),
			)
			if err != nil {
				return
			}
		} else {
			_, err := fmt.Fprintf(color.Output, "%s",
				color.New(color.FgWhite, color.Bold).Sprintf("%-15s", ip),
			)
			if err != nil {
				return
			}
		}

		i, _ := strconv.Atoi(v[0])
		if res.Hops[ttl][i].Geo.Asnumber != "" {
			// CMIN2, CUII, CN2, CUG 改为壕金色高亮
			switch {
			case res.Hops[ttl][i].Geo.Asnumber == "58807":
				fallthrough
			case res.Hops[ttl][i].Geo.Asnumber == "10099":
				fallthrough
			case res.Hops[ttl][i].Geo.Asnumber == "4809":
				fallthrough
			case res.Hops[ttl][i].Geo.Asnumber == "9929":
				fallthrough
			case res.Hops[ttl][i].Geo.Asnumber == "23764":
				fallthrough
			case res.Hops[ttl][i].Geo.Whois == "CMIN2-NET":
				fallthrough
			case strings.HasPrefix(res.Hops[ttl][i].Address.String(), "59.43."):
				_, err := fmt.Fprintf(color.Output, " %s", color.New(color.FgHiYellow, color.Bold).Sprintf("AS%-6s", res.Hops[ttl][i].Geo.Asnumber))
				if err != nil {
					return
				}
			default:
				_, err := fmt.Fprintf(color.Output, " %s", color.New(color.FgHiGreen, color.Bold).Sprintf("AS%-6s", res.Hops[ttl][i].Geo.Asnumber))
				if err != nil {
					return
				}
			}

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

			// CMIN2, CUII, CN2, CUG 改为壕金色高亮
			switch {
			case res.Hops[ttl][i].Geo.Asnumber == "58807":
				fallthrough
			case res.Hops[ttl][i].Geo.Asnumber == "10099":
				fallthrough
			case res.Hops[ttl][i].Geo.Asnumber == "4809":
				fallthrough
			case res.Hops[ttl][i].Geo.Asnumber == "9929":
				fallthrough
			case res.Hops[ttl][i].Geo.Asnumber == "23764":
				fallthrough
			case whoisFormat[0] == "[CNC-BACKBONE]":
				fallthrough
			case whoisFormat[0] == "[CUG-BACKBONE]":
				fallthrough
			case whoisFormat[0] == "[CMIN2-NET]":
				fallthrough
			case strings.HasPrefix(res.Hops[ttl][i].Address.String(), "59.43."):
				_, err := fmt.Fprintf(color.Output, " %s", color.New(color.FgHiYellow, color.Bold).Sprintf("%-16s", whoisFormat[0]))
				if err != nil {
					return
				}
			default:
				_, err := fmt.Fprintf(color.Output, " %s", color.New(color.FgHiGreen, color.Bold).Sprintf("%-16s", whoisFormat[0]))
				if err != nil {
					return
				}
			}
		}

		if len(res.Hops[ttl][i].Geo.Country) <= 1 {
			res.Hops[ttl][i].Geo.Country = "局域网"
			res.Hops[ttl][i].Geo.CountryEn = "LAN Address"
		}

		if res.Hops[ttl][i].Lang == "en" {
			if res.Hops[ttl][i].Geo.Country == "Anycast" {

			} else if res.Hops[ttl][i].Geo.Prov == "骨干网" {
				res.Hops[ttl][i].Geo.Prov = "BackBone"
			} else if res.Hops[ttl][i].Geo.ProvEn == "" {
				res.Hops[ttl][i].Geo.Country = res.Hops[ttl][i].Geo.CountryEn
			} else {
				if res.Hops[ttl][i].Geo.CityEn == "" {
					res.Hops[ttl][i].Geo.Country = res.Hops[ttl][i].Geo.ProvEn
					res.Hops[ttl][i].Geo.Prov = res.Hops[ttl][i].Geo.CountryEn
					res.Hops[ttl][i].Geo.City = ""
				} else {
					res.Hops[ttl][i].Geo.Country = res.Hops[ttl][i].Geo.CityEn
					res.Hops[ttl][i].Geo.Prov = res.Hops[ttl][i].Geo.ProvEn
					res.Hops[ttl][i].Geo.City = res.Hops[ttl][i].Geo.CountryEn
				}
			}
		}

		if net.ParseIP(ip).To4() != nil {
			_, err := fmt.Fprintf(color.Output, " %s %s %s %s %s\n    %s   ",
				color.New(color.FgWhite, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.Country),
				color.New(color.FgWhite, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.Prov),
				color.New(color.FgWhite, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.City),
				color.New(color.FgWhite, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.District),
				fmt.Sprintf("%-6s", res.Hops[ttl][i].Geo.Owner),
				color.New(color.FgHiBlack, color.Bold).Sprintf("%-39s", res.Hops[ttl][i].Hostname),
			)
			if err != nil {
				return
			}
		} else {
			_, err := fmt.Fprintf(color.Output, " %s %s %s %s %s\n    %s   ",
				color.New(color.FgWhite, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.Country),
				color.New(color.FgWhite, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.Prov),
				color.New(color.FgWhite, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.City),
				color.New(color.FgWhite, color.Bold).Sprintf("%s", res.Hops[ttl][i].Geo.District),
				fmt.Sprintf("%-6s", res.Hops[ttl][i].Geo.Owner),
				color.New(color.FgHiBlack, color.Bold).Sprintf("%-32s", res.Hops[ttl][i].Hostname),
			)
			if err != nil {
				return
			}
		}

		for j := 1; j < len(v); j++ {
			if len(v) == 2 || j == 1 {
				_, err := fmt.Fprintf(color.Output, "%s",
					color.New(color.FgHiCyan, color.Bold).Sprintf("%s", v[j]),
				)
				if err != nil {
					return
				}
			} else {
				_, err := fmt.Fprintf(color.Output, " / %s",
					color.New(color.FgHiCyan, color.Bold).Sprintf("%s", v[j]),
				)
				if err != nil {
					return
				}
			}
		}
		fmt.Println()
		blockDisplay = true
	}
}
