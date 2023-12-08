package printer

import (
	"fmt"
	"github.com/nxtrace/NTrace-core/util"
	"net"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/nxtrace/NTrace-core/trace"
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
			if util.EnableHidDstIP == "" || ip != util.DestIP {
				fmt.Fprintf(color.Output, "%s",
					color.New(color.FgWhite, color.Bold).Sprintf("%-25s", ip),
				)
			} else {
				fmt.Fprintf(color.Output, "%s",
					color.New(color.FgWhite, color.Bold).Sprintf("%-25s", util.HideIPPart(ip)),
				)
			}
		} else {
			if util.EnableHidDstIP == "" || ip != util.DestIP {
				fmt.Fprintf(color.Output, "%s",
					color.New(color.FgWhite, color.Bold).Sprintf("%-15s", ip),
				)
			} else {
				fmt.Fprintf(color.Output, "%s",
					color.New(color.FgWhite, color.Bold).Sprintf("%-15s", util.HideIPPart(ip)),
				)
			}
		}

		i, _ := strconv.Atoi(v[0])
		if res.Hops[ttl][i].Geo.Asnumber != "" {
			/*** CMIN2, CUG, CN2, CUII, CTG 改为壕金色高亮
			/* 小孩子不懂事加着玩的
			/* 此处的高亮不代表任何线路质量
			/* 仅代表走了这部分的ASN
			/* 如果使用这些ASN的IP同样会被高亮
			***/
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
			case res.Hops[ttl][i].Geo.Whois == "CTG-CN":
				fallthrough
			case res.Hops[ttl][i].Geo.Whois == "[CNC-BACKBONE]":
				fallthrough
			case res.Hops[ttl][i].Geo.Whois == "[CUG-BACKBONE]":
				fallthrough
			case res.Hops[ttl][i].Geo.Whois == "CMIN2-NET":
				fallthrough
			case strings.HasPrefix(res.Hops[ttl][i].Address.String(), "59.43."):
				fmt.Fprintf(color.Output, " %s", color.New(color.FgHiYellow, color.Bold).Sprintf("AS%-6s", res.Hops[ttl][i].Geo.Asnumber))
			default:
				fmt.Fprintf(color.Output, " %s", color.New(color.FgHiGreen, color.Bold).Sprintf("AS%-6s", res.Hops[ttl][i].Geo.Asnumber))
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
				//如果以RFC或DOD开头那么为空
				if !(strings.HasPrefix(whoisFormat[0], "RFC") ||
					strings.HasPrefix(whoisFormat[0], "DOD")) {
					whoisFormat[0] = "[" + whoisFormat[0] + "]"
				} else {
					whoisFormat[0] = ""
				}
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
			case whoisFormat[0] == "[CTG-CN]":
				fallthrough
			case whoisFormat[0] == "[CNC-BACKBONE]":
				fallthrough
			case whoisFormat[0] == "[CUG-BACKBONE]":
				fallthrough
			case whoisFormat[0] == "[CMIN2-NET]":
				fallthrough
			case strings.HasPrefix(res.Hops[ttl][i].Address.String(), "59.43."):
				fmt.Fprintf(color.Output, " %s", color.New(color.FgHiYellow, color.Bold).Sprintf("%-16s", whoisFormat[0]))
			default:
				fmt.Fprintf(color.Output, " %s", color.New(color.FgHiGreen, color.Bold).Sprintf("%-16s", whoisFormat[0]))
			}
		}

		applyLangSetting(&res.Hops[ttl][i]) // 应用语言设置

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
		for _, v := range res.Hops[ttl][i].MPLS {
			fmt.Fprintf(color.Output, "%s",
				color.New(color.FgHiBlack, color.Bold).Sprintf("\n    %s", v),
			)
		}
		fmt.Println()
		blockDisplay = true
	}
}
