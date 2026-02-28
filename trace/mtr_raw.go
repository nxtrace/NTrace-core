package trace

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"
)

// MTRRawOptions controls MTR raw streaming behavior.
type MTRRawOptions struct {
	// Interval is the delay between rounds (default: 1s).
	Interval time.Duration
	// MaxRounds is the max number of rounds. 0 means run forever until canceled.
	MaxRounds int
	// RunRound optionally overrides the traceroute call for each round.
	// It is mainly for callers that need per-round locking or global-state setup.
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

// RunMTRRaw runs continuous traceroute rounds and emits probe-level streaming records.
// Records are naturally emitted as local processing completes.
func RunMTRRaw(ctx context.Context, method Method, cfg Config, opts MTRRawOptions, onRecord MTRRawOnRecord) error {
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
