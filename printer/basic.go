package printer

import (
	"fmt"
	"github.com/nxtrace/NTrace-core/config"
	"github.com/nxtrace/NTrace-core/trace"
	"net"

	"github.com/fatih/color"
)

var version = config.Version
var buildDate = config.BuildDate
var commitID = config.CommitID

func Version() {
	fmt.Fprintf(color.Output, "%s %s %s %s\n",
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "NextTrace"),
		color.New(color.FgHiBlack, color.Bold).Sprintf("%s", version),
		color.New(color.FgHiBlack, color.Bold).Sprintf("%s", buildDate),
		color.New(color.FgHiBlack, color.Bold).Sprintf("%s", commitID),
	)
}

func CopyRight() {
	fmt.Fprintf(color.Output, "\n%s\n%s\n%s %s\n\n%s\n%s %s\n%s %s\n%s %s\n\n%s\n%s\n%s %s\n\n",
		color.New(color.FgCyan, color.Bold).Sprintf("%s", "NextTrace CopyRight"),
		color.New(color.FgGreen, color.Bold).Sprintf("%s", "NextTrace Project Creator"),
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "Leo"),
		color.New(color.FgHiBlack, color.Bold).Sprintf("%s", "i@leo.moe"),
		color.New(color.FgGreen, color.Bold).Sprintf("%s", "NextTrace Project Maintainer"),
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "Tso"),
		color.New(color.FgHiBlack, color.Bold).Sprintf("%s", "tsosunchia@gmail.com"),
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "Vincent"),
		color.New(color.FgHiBlack, color.Bold).Sprintf("%s", "i@vincent.moe"),
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "Leo"),
		color.New(color.FgHiBlack, color.Bold).Sprintf("%s", "i@leo.moe"),
		color.New(color.FgCyan, color.Bold).Sprintf("%s", "Special Acknowledgement List"),
		color.New(color.FgGreen, color.Bold).Sprintf("%s", "NextTrace Major Contributor"),
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "zhshch"),
		color.New(color.FgHiBlack, color.Bold).Sprintf("%s", "zhshch@athorx.com"),
	)

	moeQingOrgCopyRight()
	sponsor()
	//PluginCopyRight()
}

func moeQingOrgCopyRight() {
	fmt.Fprintf(color.Output, "%s\n%s %s\n%s %s\n\n",
		color.New(color.FgGreen, color.Bold).Sprintf("%s", "Supported by MoeQing Network"),
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "YekongTAT"),
		color.New(color.FgHiBlack, color.Bold).Sprintf("%s", "yekongtat@gmail.com"),
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "Haima"),
		color.New(color.FgHiBlack, color.Bold).Sprintf("%s", "haima@peers.cloud"),
	)
}

func sponsor() {
	fmt.Fprintf(color.Output, "%s\n%s\n%s\n%s\n",
		color.New(color.FgGreen, color.Bold).Sprintf("%s", "Sponsored by the following entities"),
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "DMIT.io"),
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "Misaka.io"),
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "Skywolf.cloud"),
	)
}

//func PluginCopyRight() {
//	fmt.Fprintf(color.Output, "%s\n%s %s\n\n",
//		color.New(color.FgGreen, color.Bold).Sprintf("%s", "NextTrace Map Plugin Author"),
//		color.New(color.FgWhite, color.Bold).Sprintf("%s", "Tso"),
//		color.New(color.FgHiBlack, color.Bold).Sprintf("%s", "tsosunchia@gmail.com"),
//	)
//}

func PrintTraceRouteNav(ip net.IP, domain string, dataOrigin string, maxHops int, packetSize int) {
	fmt.Println("IP Geo Data Provider: " + dataOrigin)

	if ip.String() == domain {
		fmt.Printf("traceroute to %s, %d hops max, %d bytes packets\n", ip.String(), maxHops, packetSize)
	} else {
		fmt.Printf("traceroute to %s (%s), %d hops max, %d bytes packets\n", ip.String(), domain, maxHops, packetSize)
	}
}

func applyLangSetting(h *trace.Hop) {
	if len(h.Geo.Country) <= 1 {
		//打印h.geo
		if h.Geo.Whois != "" {
			h.Geo.Country = h.Geo.Whois
		} else {
			if h.Geo.Source != "LeoMoeAPI" {
				h.Geo.Country = "网络故障"
				h.Geo.CountryEn = "Network Error"
			} else {
				h.Geo.Country = "未知"
				h.Geo.CountryEn = "Unknown"
			}
		}
	}

	if h.Lang == "en" {
		if h.Geo.Country == "Anycast" {

		} else if h.Geo.Prov == "骨干网" {
			h.Geo.Prov = "BackBone"
		} else if h.Geo.ProvEn == "" {
			h.Geo.Country = h.Geo.CountryEn
		} else {
			if h.Geo.CityEn == "" {
				h.Geo.Country = h.Geo.ProvEn
				h.Geo.Prov = h.Geo.CountryEn
				h.Geo.City = ""
			} else {
				h.Geo.Country = h.Geo.CityEn
				h.Geo.Prov = h.Geo.ProvEn
				h.Geo.City = h.Geo.CountryEn
			}
		}
	}

}
