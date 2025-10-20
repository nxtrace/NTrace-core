package server

import (
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
)

type mtrAggregator struct {
	mu    sync.Mutex
	stats map[int]*hopAccum
}

type hopAccum struct {
	TTL      int
	Host     string
	IP       string
	Sent     int
	Received int
	Sum      float64
	Last     float64
	Best     float64
	Worst    float64
	Geo      *ipgeo.IPGeoData
	Errors   map[string]int
}

type mtrHopJSON struct {
	TTL         int              `json:"ttl"`
	Host        string           `json:"host,omitempty"`
	IP          string           `json:"ip,omitempty"`
	Sent        int              `json:"sent"`
	Received    int              `json:"received"`
	LossPercent float64          `json:"loss_percent"`
	LossCount   int              `json:"loss_count"`
	Last        float64          `json:"last_ms"`
	Avg         float64          `json:"avg_ms"`
	Best        float64          `json:"best_ms"`
	Worst       float64          `json:"worst_ms"`
	Geo         *ipgeo.IPGeoData `json:"geo,omitempty"`
	FailureType string           `json:"failure_type,omitempty"`
	Errors      map[string]int   `json:"errors,omitempty"`
}

type mtrSnapshot struct {
	Iteration int          `json:"iteration"`
	Stats     []mtrHopJSON `json:"stats"`
}

func newMTRAggregator() *mtrAggregator {
	return &mtrAggregator{stats: make(map[int]*hopAccum)}
}

func (agg *mtrAggregator) Update(res *trace.Result, queries int) []mtrHopJSON {
	agg.mu.Lock()
	defer agg.mu.Unlock()

	for idx, attempts := range res.Hops {
		if len(attempts) == 0 {
			continue
		}
		ttl := idx + 1

		host := ""
		ip := ""
		var geo *ipgeo.IPGeoData
		for _, attempt := range attempts {
			if host == "" && attempt.Hostname != "" {
				host = attempt.Hostname
			}
			if ip == "" && attempt.Address != nil {
				ip = attempt.Address.String()
			}
			if geo == nil && attempt.Geo != nil {
				geo = attempt.Geo
			}
		}

		acc := agg.stats[ttl]
		if acc == nil || (ip != "" && acc.IP != "" && acc.IP != ip) {
			acc = &hopAccum{
				TTL:    ttl,
				Best:   math.MaxFloat64,
				Errors: make(map[string]int),
			}
			agg.stats[ttl] = acc
		}

		if ip != "" && acc.IP == "" {
			acc.IP = ip
		}
		if host != "" {
			acc.Host = host
		}
		if geo != nil {
			acc.Geo = geo
		}

		acc.Sent += queries
		successes := 0
		iterationErrors := make(map[string]int)

		for _, attempt := range attempts {
			if attempt.Success {
				successes++
				rttMs := float64(attempt.RTT) / float64(time.Millisecond)
				acc.Sum += rttMs
				acc.Received++
				acc.Last = rttMs
				if acc.Best == math.MaxFloat64 || rttMs < acc.Best {
					acc.Best = rttMs
				}
				if rttMs > acc.Worst {
					acc.Worst = rttMs
				}
			} else {
				key := strings.TrimSpace("timeout")
				if attempt.Error != nil {
					key = strings.TrimSpace(attempt.Error.Error())
				}
				if key == "" {
					key = "timeout"
				}
				iterationErrors[key]++
			}
		}

		remainder := queries - successes
		counted := 0
		for _, v := range iterationErrors {
			counted += v
		}
		if remainder > counted {
			iterationErrors["timeout"] += remainder - counted
		}

		for key, count := range iterationErrors {
			if acc.Errors == nil {
				acc.Errors = make(map[string]int)
			}
			acc.Errors[key] += count
		}
	}

	return agg.buildSnapshotLocked()
}

func (agg *mtrAggregator) Snapshot() []mtrHopJSON {
	agg.mu.Lock()
	defer agg.mu.Unlock()
	return agg.buildSnapshotLocked()
}

func (agg *mtrAggregator) buildSnapshotLocked() []mtrHopJSON {
	rows := make([]mtrHopJSON, 0, len(agg.stats))
	keys := make([]int, 0, len(agg.stats))
	for ttl := range agg.stats {
		keys = append(keys, ttl)
	}
	sort.Ints(keys)

	for _, ttl := range keys {
		acc := agg.stats[ttl]
		if acc == nil {
			continue
		}
		lossCount := acc.Sent - acc.Received
		lossPercent := 0.0
		if acc.Sent > 0 {
			lossPercent = float64(lossCount) / float64(acc.Sent) * 100
		}
		best := acc.Best
		if best == math.MaxFloat64 {
			best = 0
		}
		avg := 0.0
		if acc.Received > 0 {
			avg = acc.Sum / float64(acc.Received)
		}

		failureType := failureTypeFromErrors(acc.Errors, acc.Received, lossCount)

		rows = append(rows, mtrHopJSON{
			TTL:         acc.TTL,
			Host:        acc.Host,
			IP:          acc.IP,
			Sent:        acc.Sent,
			Received:    acc.Received,
			LossPercent: lossPercent,
			LossCount:   lossCount,
			Last:        acc.Last,
			Avg:         avg,
			Best:        best,
			Worst:       acc.Worst,
			Geo:         acc.Geo,
			FailureType: failureType,
			Errors:      copyErrors(acc.Errors),
		})
	}

	return rows
}

func copyErrors(src map[string]int) map[string]int {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]int, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func failureTypeFromErrors(errors map[string]int, received, lossCount int) string {
	if lossCount <= 0 {
		return ""
	}
	if len(errors) == 0 {
		if received == 0 {
			return "all_timeout"
		}
		return "partial_timeout"
	}
	allTimeout := true
	for key := range errors {
		lower := strings.ToLower(strings.TrimSpace(key))
		if lower == "timeout" || strings.Contains(lower, "timeout") {
			continue
		}
		allTimeout = false
		break
	}
	if allTimeout {
		if received == 0 {
			return "all_timeout"
		}
		return "partial_timeout"
	}
	return "mixed"
}
