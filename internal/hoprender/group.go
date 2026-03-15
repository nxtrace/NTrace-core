package hoprender

import (
	"fmt"

	"github.com/nxtrace/NTrace-core/trace"
)

type Group struct {
	IP      string
	Index   int
	Timings []string
}

func GroupHopAttempts(hops []trace.Hop) []Group {
	latestIP := ""
	indexByIP := make(map[string]int)
	groups := make([]Group, 0, len(hops))

	for i, hop := range hops {
		if hop.Address == nil {
			if latestIP != "" {
				groups[indexByIP[latestIP]].Timings = append(groups[indexByIP[latestIP]].Timings, "* ms")
			}
			continue
		}

		ip := hop.Address.String()
		groupIdx, ok := indexByIP[ip]
		if !ok {
			group := Group{
				IP:      ip,
				Index:   i,
				Timings: make([]string, 0, len(hops)),
			}
			if latestIP == "" {
				for j := 0; j < i; j++ {
					group.Timings = append(group.Timings, "* ms")
				}
			}
			groups = append(groups, group)
			groupIdx = len(groups) - 1
			indexByIP[ip] = groupIdx
		}

		groups[groupIdx].Timings = append(groups[groupIdx].Timings, fmt.Sprintf("%.2f ms", hop.RTT.Seconds()*1000))
		latestIP = ip
	}

	if latestIP == "" {
		return nil
	}
	return groups
}
