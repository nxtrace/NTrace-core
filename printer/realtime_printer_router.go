package printer

import (
	"fmt"

	"github.com/fatih/color"

	"github.com/nxtrace/NTrace-core/internal/hoprender"
	"github.com/nxtrace/NTrace-core/trace"
)

func RealtimePrinterWithRouter(res *trace.Result, ttl int) {
	printRealtimeTTL(ttl)
	groups := hoprender.GroupHopAttempts(res.Hops[ttl])
	if len(groups) == 0 {
		printRealtimeEmptyHop()
		return
	}

	blockDisplay := false
	for _, group := range groups {
		renderRouterHopLine(res, ttl, group, blockDisplay)
		if !blockDisplay {
			renderRouterSummary(res, ttl)
		}
		blockDisplay = true
	}
}

func renderRouterSummary(res *trace.Result, ttl int) {
	hop := &res.Hops[ttl][0]
	if hop.Geo == nil {
		return
	}

	fmt.Fprintf(color.Output, "%s   %s %s %s   %s\n",
		color.New(color.FgWhite, color.Bold).Sprintf("-"),
		color.New(color.FgHiYellow, color.Bold).Sprintf("%s", hop.Geo.Prefix),
		color.New(color.FgWhite, color.Bold).Sprintf("路由表"),
		color.New(color.FgHiCyan, color.Bold).Sprintf("Beta"),
		color.New(color.FgWhite, color.Bold).Sprintf("-"),
	)
	GetRouter(&hop.Geo.Router, "AS"+hop.Geo.Asnumber)
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
