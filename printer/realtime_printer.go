package printer

import (
	"github.com/nxtrace/NTrace-core/internal/hoprender"
	"github.com/nxtrace/NTrace-core/trace"
)

func RealtimePrinter(res *trace.Result, ttl int) {
	printRealtimeTTL(ttl)
	groups := hoprender.GroupHopAttempts(res.Hops[ttl])
	if len(groups) == 0 {
		printRealtimeEmptyHop()
		return
	}

	blockDisplay := false
	for _, group := range groups {
		renderRealtimeHopLine(res, ttl, group, blockDisplay)
		blockDisplay = true
	}
}
