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

var lookupMTRPTR = lookupPTR

func lookupMTRMetadata(addr net.Addr, cfg Config) mtrMetadataPatch {
	ipStr := strings.TrimSpace(mtrAddrString(addr))
	if ipStr == "" {
		return mtrMetadataPatch{}
	}

	hostPatch := lookupMTRHostMetadata(addr, cfg)
	geoPatch := lookupMTRGeoMetadata(addr, cfg, hostPatch.host)
	return mtrMetadataPatch{
		ip:   ipStr,
		host: hostPatch.host,
		geo:  geoPatch.geo,
	}
}

func lookupMTRHostMetadata(addr net.Addr, cfg Config) mtrMetadataPatch {
	ipStr := strings.TrimSpace(mtrAddrString(addr))
	if ipStr == "" {
		return mtrMetadataPatch{}
	}
	if !cfg.RDNS {
		return mtrMetadataPatch{ip: ipStr}
	}

	host := ""
	ptrs := lookupMTRPTR(cfg.Context, ipStr)
	if len(ptrs) > 0 {
		host = CanonicalHostname(ptrs[0])
	}
	return mtrMetadataPatch{ip: ipStr, host: host}
}

func lookupMTRGeoMetadata(addr net.Addr, cfg Config, host string) mtrMetadataPatch {
	ipStr := strings.TrimSpace(mtrAddrString(addr))
	if ipStr == "" {
		return mtrMetadataPatch{}
	}

	var geo *ipgeo.IPGeoData
	if cfg.IPGeoSource != nil {
		if cfg.DN42 {
			query := ipStr
			if strings.TrimSpace(host) != "" {
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
		ip:  ipStr,
		geo: geo,
	}
}
