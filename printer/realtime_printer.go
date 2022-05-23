package printer

import (
	"fmt"

	"github.com/xgadget-lab/nexttrace/trace"
)

func RealtimePrinter(res *trace.Result, ttl int) {
	fmt.Print(ttl)
	for i := range res.Hops[ttl] {
		HopPrinter(res.Hops[ttl][i])
	}

}
