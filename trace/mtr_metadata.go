package trace

import (
	"net"
	"strings"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

type mtrMetadataPatch struct {
	ip   string
	host string
	geo  *ipgeo.IPGeoData
}

func lookupMTRMetadata(addr net.Addr, cfg Config) mtrMetadataPatch {
	ipStr := strings.TrimSpace(mtrAddrString(addr))
	if ipStr == "" {
		return mtrMetadataPatch{}
	}

	host := ""
	if cfg.RDNS {
		ptrs := lookupPTR(cfg.Context, ipStr)
		if len(ptrs) > 0 {
			host = CanonicalHostname(ptrs[0])
		}
	}

	var geo *ipgeo.IPGeoData
	if cfg.IPGeoSource != nil {
		if cfg.DN42 {
			query := ipStr
			if host != "" {
				query = ipStr + "," + host
			}
			if resolved, err := lookupGeoWithRetry(cfg, query, query, true); err == nil {
				geo = resolved
			}
		} else if g, ok := ipgeo.Filter(ipStr); ok {
			geo = g
		} else if resolved, err := lookupGeoWithRetry(cfg, ipStr, ipStr, false); err == nil {
			geo = resolved
		}
	}

	return mtrMetadataPatch{
		ip:   ipStr,
		host: host,
		geo:  geo,
	}
}
