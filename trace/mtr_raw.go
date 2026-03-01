package trace

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/nxtrace/NTrace-core/util"
)

// MTRRawOptions controls MTR raw streaming behavior.
type MTRRawOptions struct {
	// Interval is the delay between rounds (default: 1s).
	// Legacy round-based mode only.
	Interval time.Duration
	// MaxRounds is the max number of rounds. 0 means run forever until canceled.
	// Legacy round-based mode only.
	MaxRounds int
	// HopInterval is the per-hop probe interval (per-hop scheduling mode).
	// > 0 activates per-hop scheduling; Interval/MaxRounds are ignored.
	HopInterval time.Duration
	// MaxPerHop is the max probes per TTL in per-hop mode. 0 = unlimited.
	MaxPerHop int
	// RunRound optionally overrides the traceroute call for each round.
	// It is mainly for callers that need per-round locking or global-state setup.
	// Legacy round-based mode only.
	RunRound func(method Method, cfg Config) (*Result, error)
}

// MTRRawRecord is one stream record emitted by MTR raw mode.
// It keeps the same information family as classic --raw output.
type MTRRawRecord struct {
	Iteration int      `json:"iteration"`
	TTL       int      `json:"ttl"`
	Success   bool     `json:"success"`
	IP        string   `json:"ip,omitempty"`
	Host      string   `json:"host,omitempty"`
	RTTMs     float64  `json:"rtt_ms"`
	ASN       string   `json:"asn,omitempty"`
	Country   string   `json:"country,omitempty"`
	Prov      string   `json:"prov,omitempty"`
	City      string   `json:"city,omitempty"`
	District  string   `json:"district,omitempty"`
	Owner     string   `json:"owner,omitempty"`
	Lat       float64  `json:"lat"`
	Lng       float64  `json:"lng"`
	MPLS      []string `json:"mpls,omitempty"`
}

// MTRRawOnRecord is called for each probe event.
type MTRRawOnRecord func(rec MTRRawRecord)

var mtrRawTracerouteFn = Traceroute

// RunMTRRaw runs continuous traceroute and emits probe-level streaming records.
//
// When opts.HopInterval > 0, uses per-hop scheduling (each TTL independent);
// otherwise uses legacy round-based scheduling for backward compatibility.
func RunMTRRaw(ctx context.Context, method Method, cfg Config, opts MTRRawOptions, onRecord MTRRawOnRecord) error {
	if opts.HopInterval > 0 {
		return runMTRRawPerHop(ctx, method, cfg, opts, onRecord)
	}
	return runMTRRawRoundBased(ctx, method, cfg, opts, onRecord)
}

// runMTRRawPerHop uses per-hop scheduling for raw streaming.
func runMTRRawPerHop(ctx context.Context, method Method, cfg Config, opts MTRRawOptions, onRecord MTRRawOnRecord) error {
	roundCfg := cfg
	roundCfg.NumMeasurements = 1
	roundCfg.MaxAttempts = 1
	roundCfg.AsyncPrinter = nil
	roundCfg.RealtimePrinter = nil

	if roundCfg.MaxHops == 0 {
		roundCfg.MaxHops = 30
	}
	if roundCfg.ICMPMode <= 0 && util.EnvICMPMode > 0 {
		roundCfg.ICMPMode = util.EnvICMPMode
	}
	switch roundCfg.ICMPMode {
	case 0, 1, 2:
	default:
		roundCfg.ICMPMode = 0
	}

	var prober mtrTTLProber

	if method == ICMPTrace {
		engine, err := newMTRICMPEngine(roundCfg)
		if err != nil {
			return fmt.Errorf("mtr raw: %w", err)
		}
		if err := engine.start(ctx); err != nil {
			return fmt.Errorf("mtr raw: %w", err)
		}
		prober = engine
	} else {
		prober = &mtrFallbackTTLProber{method: method, config: roundCfg}
	}

	agg := NewMTRAggregator()

	return runMTRScheduler(ctx, prober, agg, mtrSchedulerConfig{
		BeginHop:         roundCfg.BeginHop,
		MaxHops:          roundCfg.MaxHops,
		HopInterval:      opts.HopInterval,
		MaxPerHop:        opts.MaxPerHop,
		ParallelRequests: roundCfg.ParallelRequests,
		FillGeo:          true,
		BaseConfig:       roundCfg,
		DstIP:            roundCfg.DstIP,
	}, nil, func(result mtrProbeResult, iteration int) {
		if onRecord == nil {
			return
		}
		rec := buildMTRRawRecordFromProbe(iteration, result, roundCfg)
		onRecord(rec)
	})
}

// runMTRRawRoundBased is the legacy round-based raw streaming path.
func runMTRRawRoundBased(ctx context.Context, method Method, cfg Config, opts MTRRawOptions, onRecord MTRRawOnRecord) error {
	if opts.Interval <= 0 {
		opts.Interval = time.Second
	}

	roundCfg := cfg
	roundCfg.NumMeasurements = 1
	roundCfg.MaxAttempts = 1
	roundCfg.AsyncPrinter = nil
	roundCfg.RealtimePrinter = nil

	runRound := opts.RunRound
	if runRound == nil {
		runRound = mtrRawTracerouteFn
	}

	iteration := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		iteration++
		seen := make(map[int]int)
		var seenMu sync.Mutex

		cfgForRound := roundCfg
		cfgForRound.RealtimePrinter = func(res *Result, ttl int) {
			if onRecord == nil || ctx.Err() != nil {
				return
			}
			if ttl < 0 || ttl >= len(res.Hops) {
				return
			}

			seenMu.Lock()
			start := seen[ttl]
			end := len(res.Hops[ttl])
			seen[ttl] = end
			seenMu.Unlock()

			if start >= end {
				return
			}

			for i := start; i < end; i++ {
				h := res.Hops[ttl][i]
				rec := buildMTRRawRecord(iteration, h, cfgForRound)
				onRecord(rec)
			}
		}

		done := make(chan struct{})
		var traceErr error
		go func() {
			_, traceErr = runRound(method, cfgForRound)
			close(done)
		}()

		select {
		case <-ctx.Done():
			// Wait for the in-flight round to finish before returning, so callers
			// can safely release any per-round/global state after RunMTRRaw exits.
			<-done
			return ctx.Err()
		case <-done:
		}

		if traceErr != nil {
			return traceErr
		}

		if opts.MaxRounds > 0 && iteration >= opts.MaxRounds {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(opts.Interval):
		}
	}
}

func buildMTRRawRecord(iteration int, h Hop, cfg Config) MTRRawRecord {
	rec := MTRRawRecord{
		Iteration: iteration,
		TTL:       h.TTL,
		Success:   h.Success && h.Address != nil,
	}

	if h.Address != nil {
		rec.IP = addrToIPString(h.Address)
	}
	rec.Host = strings.TrimSpace(h.Hostname)
	if h.RTT > 0 {
		rec.RTTMs = float64(h.RTT) / float64(time.Millisecond)
	}
	if len(h.MPLS) > 0 {
		rec.MPLS = append([]string(nil), h.MPLS...)
	}

	if h.Address != nil && (h.Geo == nil || isPendingGeo(h.Geo)) {
		if h.Lang == "" {
			h.Lang = cfg.Lang
		}
		_ = h.fetchIPData(cfg)
	}

	if h.Geo != nil {
		rec.ASN = strings.TrimSpace(h.Geo.Asnumber)
		rec.Country = geoTextByLang(cfg.Lang, h.Geo.Country, h.Geo.CountryEn)
		rec.Prov = geoTextByLang(cfg.Lang, h.Geo.Prov, h.Geo.ProvEn)
		rec.City = geoTextByLang(cfg.Lang, h.Geo.City, h.Geo.CityEn)
		rec.District = strings.TrimSpace(h.Geo.District)
		rec.Owner = strings.TrimSpace(h.Geo.Owner)
		if rec.Owner == "" {
			rec.Owner = strings.TrimSpace(h.Geo.Isp)
		}
		rec.Lat = h.Geo.Lat
		rec.Lng = h.Geo.Lng
	}

	return rec
}

func addrToIPString(addr net.Addr) string {
	switch v := addr.(type) {
	case *net.IPAddr:
		if v.IP != nil {
			return v.IP.String()
		}
	case *net.UDPAddr:
		if v.IP != nil {
			return v.IP.String()
		}
	case *net.TCPAddr:
		if v.IP != nil {
			return v.IP.String()
		}
	}
	s := strings.TrimSpace(addr.String())
	if host, _, err := net.SplitHostPort(s); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(s, "[]")
}

func geoTextByLang(lang, cn, en string) string {
	cn = strings.TrimSpace(cn)
	en = strings.TrimSpace(en)
	if strings.EqualFold(lang, "en") {
		if en != "" {
			return en
		}
		return cn
	}
	if cn != "" {
		return cn
	}
	return en
}

// buildMTRRawRecordFromProbe constructs an MTRRawRecord from a per-hop scheduler probe result.
func buildMTRRawRecordFromProbe(iteration int, pr mtrProbeResult, cfg Config) MTRRawRecord {
	rec := MTRRawRecord{
		Iteration: iteration,
		TTL:       pr.TTL,
		Success:   pr.Success && pr.Addr != nil,
	}

	if pr.Addr != nil {
		rec.IP = addrToIPString(pr.Addr)
	}
	if pr.RTT > 0 {
		rec.RTTMs = float64(pr.RTT) / float64(time.Millisecond)
	}
	if len(pr.MPLS) > 0 {
		rec.MPLS = append([]string(nil), pr.MPLS...)
	}

	if pr.Addr == nil {
		return rec
	}

	// Use pre-resolved data from fallback prober when available;
	// otherwise call fetchIPData (ICMP path, or when prober didn't fill).
	if pr.Geo != nil || pr.Hostname != "" {
		if pr.Geo != nil {
			rec.ASN = strings.TrimSpace(pr.Geo.Asnumber)
			rec.Country = geoTextByLang(cfg.Lang, pr.Geo.Country, pr.Geo.CountryEn)
			rec.Prov = geoTextByLang(cfg.Lang, pr.Geo.Prov, pr.Geo.ProvEn)
			rec.City = geoTextByLang(cfg.Lang, pr.Geo.City, pr.Geo.CityEn)
			rec.District = strings.TrimSpace(pr.Geo.District)
			rec.Owner = strings.TrimSpace(pr.Geo.Owner)
			if rec.Owner == "" {
				rec.Owner = strings.TrimSpace(pr.Geo.Isp)
			}
			rec.Lat = pr.Geo.Lat
			rec.Lng = pr.Geo.Lng
		}
		if pr.Hostname != "" {
			rec.Host = strings.TrimSpace(pr.Hostname)
		}
	} else if cfg.IPGeoSource != nil || cfg.RDNS {
		// Fallback: fetch geo/PTR via fetchIPData (handles RDNS even when IPGeoSource is nil).
		h := Hop{Address: pr.Addr, Lang: cfg.Lang}
		_ = h.fetchIPData(cfg)
		if h.Geo != nil {
			rec.ASN = strings.TrimSpace(h.Geo.Asnumber)
			rec.Country = geoTextByLang(cfg.Lang, h.Geo.Country, h.Geo.CountryEn)
			rec.Prov = geoTextByLang(cfg.Lang, h.Geo.Prov, h.Geo.ProvEn)
			rec.City = geoTextByLang(cfg.Lang, h.Geo.City, h.Geo.CityEn)
			rec.District = strings.TrimSpace(h.Geo.District)
			rec.Owner = strings.TrimSpace(h.Geo.Owner)
			if rec.Owner == "" {
				rec.Owner = strings.TrimSpace(h.Geo.Isp)
			}
			rec.Lat = h.Geo.Lat
			rec.Lng = h.Geo.Lng
		}
		if h.Hostname != "" {
			rec.Host = strings.TrimSpace(h.Hostname)
		}
	}

	return rec
}
