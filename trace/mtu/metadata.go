package mtu

import (
	"context"
	"reflect"
	"strings"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/util"
)

const mtuTimeoutGeoSource = "timeout"

var mtuLookupAddr = util.LookupAddrWithContext

type mtuGeoLookupResult struct {
	geo *ipgeo.IPGeoData
	err error
}

func enrichHopMetadata(ctx context.Context, cfg Config, hop Hop) (Hop, bool) {
	if !shouldFetchHopMetadata(cfg, hop) {
		return hop, false
	}

	updated := hop
	if ctx != nil && ctx.Err() != nil {
		return updated, false
	}
	ipStr := strings.TrimSpace(hop.IP)
	geoCh := startMTUGeoLookup(cfg, ipStr)
	rDNSStarted := cfg.RDNS && updated.Hostname == ""
	var rDNSCh <-chan []string
	if rDNSStarted {
		rDNSCh = startMTUPTRLookup(ctx, ipStr)
	}

	updated = waitForMTUGeoAndPTR(ctx, cfg, updated, geoCh, rDNSStarted, rDNSCh)
	return updated, !reflect.DeepEqual(updated, hop)
}

func shouldFetchHopMetadata(cfg Config, hop Hop) bool {
	if strings.TrimSpace(hop.IP) == "" || hop.Event == EventTimeout {
		return false
	}
	return cfg.IPGeoSource != nil || cfg.RDNS
}

func startMTUPTRLookup(ctx context.Context, ipStr string) <-chan []string {
	ch := make(chan []string, 1)
	go func() {
		ptrs, err := mtuLookupAddr(ctx, ipStr)
		if err != nil {
			ch <- nil
			return
		}
		ch <- ptrs
	}()
	return ch
}

func applyMTUPTRResult(h *Hop, ptrs []string) {
	if len(ptrs) == 0 {
		return
	}
	h.Hostname = strings.TrimSuffix(strings.TrimSpace(ptrs[0]), ".")
}

func startMTUGeoLookup(cfg Config, ipStr string) <-chan mtuGeoLookupResult {
	if cfg.IPGeoSource == nil {
		if cfg.RDNS {
			return nil
		}
		ch := make(chan mtuGeoLookupResult, 1)
		ch <- mtuGeoLookupResult{}
		return ch
	}
	ch := make(chan mtuGeoLookupResult, 1)
	go func() {
		if geo, ok := ipgeo.Filter(ipStr); ok {
			ch <- mtuGeoLookupResult{geo: normalizeMTUGeoData(geo)}
			return
		}

		geo, err := cfg.IPGeoSource(ipStr, cfg.Timeout, cfg.Lang, false)
		if err != nil {
			ch <- mtuGeoLookupResult{geo: mtuTimeoutGeo(), err: err}
			return
		}
		ch <- mtuGeoLookupResult{geo: normalizeMTUGeoData(geo)}
	}()
	return ch
}

func waitForMTUGeoAndPTR(ctx context.Context, cfg Config, hop Hop, geoCh <-chan mtuGeoLookupResult, rDNSStarted bool, rDNSCh <-chan []string) Hop {
	applyGeo := func(res mtuGeoLookupResult) {
		if res.geo != nil {
			hop.Geo = res.geo
		}
	}

	if cfg.AlwaysWaitRDNS {
		if rDNSStarted {
			select {
			case ptrs := <-rDNSCh:
				applyMTUPTRResult(&hop, ptrs)
			case <-time.After(time.Second):
			case <-ctxDoneChan(ctx):
			}
		}
		if geoCh != nil {
			select {
			case res := <-geoCh:
				applyGeo(res)
			case <-ctxDoneChan(ctx):
			}
		}
		return hop
	}

	if rDNSStarted {
		if geoCh == nil {
			select {
			case ptrs := <-rDNSCh:
				applyMTUPTRResult(&hop, ptrs)
			case <-ctxDoneChan(ctx):
			}
			return hop
		}
		select {
		case res := <-geoCh:
			applyGeo(res)
			return hop
		case ptrs := <-rDNSCh:
			applyMTUPTRResult(&hop, ptrs)
			select {
			case res := <-geoCh:
				applyGeo(res)
			case <-ctxDoneChan(ctx):
			}
			return hop
		case <-ctxDoneChan(ctx):
			return hop
		}
	}

	if geoCh != nil {
		select {
		case res := <-geoCh:
			applyGeo(res)
		case <-ctxDoneChan(ctx):
		}
	}
	return hop
}

func ctxDoneChan(ctx context.Context) <-chan struct{} {
	if ctx == nil {
		return nil
	}
	return ctx.Done()
}

func normalizeMTUGeoData(geo *ipgeo.IPGeoData) *ipgeo.IPGeoData {
	if geo == nil {
		return nil
	}
	if geo.Source == mtuTimeoutGeoSource {
		return geo
	}
	if geo.Asnumber == "" &&
		geo.Country == "" &&
		geo.CountryEn == "" &&
		geo.Prov == "" &&
		geo.ProvEn == "" &&
		geo.City == "" &&
		geo.CityEn == "" &&
		geo.District == "" &&
		geo.Owner == "" &&
		geo.Isp == "" &&
		geo.Domain == "" &&
		geo.Whois == "" &&
		geo.Lat == 0 &&
		geo.Lng == 0 &&
		geo.Prefix == "" &&
		len(geo.Router) == 0 &&
		geo.Source == "" {
		return nil
	}
	return geo
}

func mtuTimeoutGeo() *ipgeo.IPGeoData {
	return &ipgeo.IPGeoData{
		Country:   "网络故障",
		CountryEn: "Network Error",
		Source:    mtuTimeoutGeoSource,
	}
}
