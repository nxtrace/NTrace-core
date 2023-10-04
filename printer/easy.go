package printer

import (
	"fmt"

	"github.com/nxtrace/NTrace-core/trace"
)

func EasyPrinter(res *trace.Result, ttl int) {
	for i := range res.Hops[ttl] {
		if res.Hops[ttl][i].Address == nil {
			fmt.Printf("%d|*||||||\n", ttl+1)
			continue
		}
		applyLangSetting(&res.Hops[ttl][i]) // 应用语言设置
		fmt.Printf(
			"%d|%s|%s|%.2f|%s|%s|%s|%s|%s|%s|%.4f|%.4f\n",
			ttl+1,
			res.Hops[ttl][i].Address.String(),
			res.Hops[ttl][i].Hostname,
			float64(res.Hops[ttl][i].RTT.Microseconds())/1000,
			res.Hops[ttl][i].Geo.Asnumber,
			res.Hops[ttl][i].Geo.Country,
			res.Hops[ttl][i].Geo.Prov,
			res.Hops[ttl][i].Geo.City,
			res.Hops[ttl][i].Geo.District,
			res.Hops[ttl][i].Geo.Owner,
			res.Hops[ttl][i].Geo.Lat,
			res.Hops[ttl][i].Geo.Lng,
		)
	}
}
