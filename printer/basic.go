package printer

import (
	"fmt"
	"net"

	"github.com/fatih/color"
)

var version = "v0.0.0.alpha"
var buildDate = ""
var commitID = ""

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

	MoeQingOrgCopyRight()
	PluginCopyRight()
}

func MoeQingOrgCopyRight() {
	fmt.Fprintf(color.Output, "%s\n%s %s\n%s %s\n\n",
		color.New(color.FgHiYellow, color.Bold).Sprintf("%s", "MoeQing Network"),
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "YekongTAT"),
		color.New(color.FgHiBlack, color.Bold).Sprintf("%s", "yekongtat@gmail.com"),
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "Haima"),
		color.New(color.FgHiBlack, color.Bold).Sprintf("%s", "haima@peers.cloud"),
	)
}

func PluginCopyRight() {
	fmt.Fprintf(color.Output, "%s\n%s %s\n\n",
		color.New(color.FgGreen, color.Bold).Sprintf("%s", "NextTrace Map Plugin Author"),
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "Tso"),
		color.New(color.FgHiBlack, color.Bold).Sprintf("%s", "tsosunchia@gmail.com"),
	)
}

func PrintTraceRouteNav(ip net.IP, domain string, dataOrigin string) {
	fmt.Println("IP Geo Data Provider: " + dataOrigin)

	if ip.String() == domain {
		fmt.Printf("traceroute to %s, 30 hops max, 32 byte packets\n", ip.String())
	} else {
		fmt.Printf("traceroute to %s (%s), 30 hops max, 32 byte packets\n", ip.String(), domain)
	}
}
