package printer

import (
	"fmt"
	"net"
	"strings"

	"github.com/fatih/color"

	"github.com/nxtrace/NTrace-core/internal/hoprender"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/util"
)

func printRealtimeTTL(ttl int) {
	fmt.Printf("%s  ", color.New(color.FgHiYellow, color.Bold).Sprintf("%-2d", ttl+1))
}

func printRealtimeEmptyHop() {
	fmt.Fprintf(color.Output, "%s\n", color.New(color.FgWhite, color.Bold).Sprintf("*"))
}

func displayRealtimeIP(ip string) string {
	if util.EnableHidDstIP && ip == util.DstIP {
		return util.HideIPPart(ip)
	}
	return ip
}

func printRealtimeIPColumn(ip string) bool {
	isIPv6 := net.ParseIP(ip).To4() == nil
	width := "%-15s"
	if isIPv6 {
		width = "%-25s"
	}
	fmt.Fprintf(color.Output, "%s", color.New(color.FgWhite, color.Bold).Sprintf(width, displayRealtimeIP(ip)))
	return isIPv6
}

func ensureHopGeo(hop *trace.Hop) {
	if hop.Geo == nil {
		hop.Geo = &ipgeo.IPGeoData{}
	}
}

func formatWhoisPrefix(whois string, suppressReserved bool) string {
	whoisFormat := strings.Split(whois, "-")
	if len(whoisFormat) > 1 {
		whoisFormat[0] = strings.Join(whoisFormat[:2], "-")
	}
	prefix := whoisFormat[0]
	if prefix == "" {
		return ""
	}
	if suppressReserved && (strings.HasPrefix(prefix, "RFC") || strings.HasPrefix(prefix, "DOD")) {
		return ""
	}
	return "[" + prefix + "]"
}

func highlightRealtimeBackbone(hop *trace.Hop, whoisPrefix string) bool {
	switch {
	case hop.Geo.Asnumber == "58807":
		return true
	case hop.Geo.Asnumber == "10099":
		return true
	case hop.Geo.Asnumber == "4809":
		return true
	case hop.Geo.Asnumber == "9929":
		return true
	case hop.Geo.Asnumber == "23764":
		return true
	case whoisPrefix == "[CTG-CN]":
		return true
	case whoisPrefix == "[CNC-BACKBONE]":
		return true
	case whoisPrefix == "[CUG-BACKBONE]":
		return true
	case whoisPrefix == "[CMIN2-NET]":
		return true
	case hop.Address != nil && strings.HasPrefix(hop.Address.String(), "59.43."):
		return true
	default:
		return false
	}
}

func printASNColumn(hop *trace.Hop, highlight bool) {
	if hop.Geo.Asnumber == "" {
		fmt.Printf(" %-8s", "*")
		return
	}
	style := color.New(color.FgHiGreen, color.Bold)
	if highlight {
		style = color.New(color.FgHiYellow, color.Bold)
	}
	fmt.Fprintf(color.Output, " %s", style.Sprintf("AS%-6s", hop.Geo.Asnumber))
}

func printWhoisColumn(prefix string, highlight bool) {
	style := color.New(color.FgHiGreen, color.Bold)
	if highlight {
		style = color.New(color.FgHiYellow, color.Bold)
	}
	fmt.Fprintf(color.Output, " %s", style.Sprintf("%-16s", prefix))
}

func displayRealtimeHostname(ip, hostname string) string {
	if util.EnableHidDstIP && ip == util.DstIP {
		return ""
	}
	return hostname
}

func printLocationLine(hop *trace.Hop, ip string, isIPv6 bool) {
	hostname := displayRealtimeHostname(ip, hop.Hostname)
	template := " %s %s %s %s %s\n    %s   "
	hostWidth := "%-39s"
	if isIPv6 {
		hostWidth = "%-32s"
	}
	fmt.Fprintf(color.Output, template,
		color.New(color.FgWhite, color.Bold).Sprintf("%s", hop.Geo.Country),
		color.New(color.FgWhite, color.Bold).Sprintf("%s", hop.Geo.Prov),
		color.New(color.FgWhite, color.Bold).Sprintf("%s", hop.Geo.City),
		color.New(color.FgWhite, color.Bold).Sprintf("%s", hop.Geo.District),
		fmt.Sprintf("%-6s", hop.Geo.Owner),
		color.New(color.FgHiBlack, color.Bold).Sprintf(hostWidth, hostname),
	)
}

func printTimingSeries(values []string) {
	for i, value := range values {
		if i == 0 {
			fmt.Fprintf(color.Output, "%s", color.New(color.FgHiCyan, color.Bold).Sprintf("%s", value))
			continue
		}
		fmt.Fprintf(color.Output, " / %s", color.New(color.FgHiCyan, color.Bold).Sprintf("%s", value))
	}
}

func printHopMPLS(labels []string) {
	for _, label := range labels {
		fmt.Fprintf(color.Output, "%s", color.New(color.FgHiBlack, color.Bold).Sprintf("\n    %s", label))
	}
}

func renderRealtimeHopLine(res *trace.Result, ttl int, group hoprender.Group, blockDisplay bool) {
	if blockDisplay {
		fmt.Printf("%4s", "")
	}

	hop := &res.Hops[ttl][group.Index]
	ensureHopGeo(hop)
	applyLangSetting(hop)

	isIPv6 := printRealtimeIPColumn(group.IP)
	whoisPrefix := formatWhoisPrefix(hop.Geo.Whois, true)
	highlight := highlightRealtimeBackbone(hop, whoisPrefix)
	printASNColumn(hop, highlight)
	if !isIPv6 {
		printWhoisColumn(whoisPrefix, highlight)
	}
	printLocationLine(hop, group.IP, isIPv6)
	printTimingSeries(group.Timings)
	printHopMPLS(hop.MPLS)
	fmt.Println()
}

func prepareRouterGeo(hop *trace.Hop) {
	ensureHopGeo(hop)
	if hop.Geo.Country == "" && hop.Geo.Source != trace.PendingGeoSource {
		hop.Geo.Country = "LAN Address"
	}
	applyLangSetting(hop)
}

func renderRouterHopLine(res *trace.Result, ttl int, group hoprender.Group, blockDisplay bool) {
	if blockDisplay {
		fmt.Printf("%4s", "")
	}

	hop := &res.Hops[ttl][group.Index]
	prepareRouterGeo(hop)

	isIPv6 := printRealtimeIPColumn(group.IP)
	printASNColumn(hop, false)
	if !isIPv6 {
		printWhoisColumn(formatWhoisPrefix(hop.Geo.Whois, false), false)
	}
	printLocationLine(hop, group.IP, isIPv6)
	printTimingSeries(group.Timings)
	fmt.Println()
}
