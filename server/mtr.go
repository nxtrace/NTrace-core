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
	mu        sync.Mutex
	stats     map[int]map[string]*hopAccum
	nextOrder int
}

type hopAccum struct {
	TTL      int
	Key      string
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
	order    int
	mplsSet  map[string]struct{}
}

type groupMetrics struct {
	host     string
	ip       string
	geo      *ipgeo.IPGeoData
	sum      float64
	last     float64
	best     float64
	worst    float64
	received int
	count    int
	errors   map[string]int
	mpls     map[string]struct{}
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
	MPLS        []string         `json:"mpls,omitempty"`
}

type mtrSnapshot struct {
	Iteration int          `json:"iteration"`
	Stats     []mtrHopJSON `json:"stats"`
}

func newMTRAggregator() *mtrAggregator {
	return &mtrAggregator{
		stats: make(map[int]map[string]*hopAccum),
	}
}

func (agg *mtrAggregator) Update(res *trace.Result, queries int) []mtrHopJSON {
	agg.mu.Lock()
	defer agg.mu.Unlock()

	if queries <= 0 {
		queries = 1
	}

	for idx, attempts := range res.Hops {
		if len(attempts) == 0 {
			continue
		}
		ttl := idx + 1
		accMap := agg.stats[ttl]
		if accMap == nil {
			accMap = make(map[string]*hopAccum)
			agg.stats[ttl] = accMap
		}

		groups := make(map[string]*groupMetrics)
		for _, attempt := range attempts {
			host := strings.TrimSpace(attempt.Hostname)
			var ip string
			if attempt.Address != nil {
				ip = strings.TrimSpace(attempt.Address.String())
			}
			key := hopKey(ip, host)
			group := groups[key]
			if group == nil {
				group = &groupMetrics{
					host: host,
					ip:   ip,
					best: math.MaxFloat64,
				}
				groups[key] = group
			}
			group.count++
			if group.geo == nil && attempt.Geo != nil {
				group.geo = attempt.Geo
			}
			if len(attempt.MPLS) > 0 {
				if group.mpls == nil {
					group.mpls = make(map[string]struct{})
				}
				for _, label := range attempt.MPLS {
					val := strings.TrimSpace(label)
					if val != "" {
						group.mpls[val] = struct{}{}
					}
				}
			}
			if attempt.Success {
				rttMs := float64(attempt.RTT) / float64(time.Millisecond)
				group.sum += rttMs
				group.received++
				group.last = rttMs
				if rttMs > group.worst {
					group.worst = rttMs
				}
				if rttMs > 0 && rttMs < group.best {
					group.best = rttMs
				}
			} else {
				errKey := strings.TrimSpace("timeout")
				if attempt.Error != nil {
					errKey = strings.TrimSpace(attempt.Error.Error())
				}
				if errKey == "" {
					errKey = "timeout"
				}
				if group.errors == nil {
					group.errors = make(map[string]int)
				}
				group.errors[errKey]++
			}
		}

		for key, group := range groups {
			acc := accMap[key]
			if acc == nil {
				acc = &hopAccum{
					TTL:     ttl,
					Key:     key,
					Best:    math.MaxFloat64,
					order:   agg.nextOrder,
					mplsSet: make(map[string]struct{}),
				}
				agg.nextOrder++
				accMap[key] = acc
			}

			if group.ip != "" {
				acc.IP = group.ip
			}
			if group.host != "" {
				acc.Host = group.host
			}
			if group.geo != nil {
				acc.Geo = group.geo
			}

			acc.Sent += group.count

			if group.received > 0 {
				acc.Sum += group.sum
				acc.Received += group.received
				acc.Last = group.last
				if group.best > 0 && (acc.Best == math.MaxFloat64 || group.best < acc.Best) {
					acc.Best = group.best
				}
				if group.worst > acc.Worst {
					acc.Worst = group.worst
				}
			}

			if len(group.errors) > 0 {
				if acc.Errors == nil {
					acc.Errors = make(map[string]int)
				}
				for errKey, count := range group.errors {
					acc.Errors[errKey] += count
				}
			}
			if len(group.mpls) > 0 {
				if acc.mplsSet == nil {
					acc.mplsSet = make(map[string]struct{})
				}
				for label := range group.mpls {
					acc.mplsSet[label] = struct{}{}
				}
			}
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
		accMap := agg.stats[ttl]
		if len(accMap) == 0 {
			continue
		}
		accs := make([]*hopAccum, 0, len(accMap))
		for _, acc := range accMap {
			accs = append(accs, acc)
		}
		sort.SliceStable(accs, func(i, j int) bool {
			if accs[i].order == accs[j].order {
				return accs[i].IP < accs[j].IP
			}
			return accs[i].order < accs[j].order
		})

		for _, acc := range accs {
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
			mpls := sortedSet(acc.mplsSet)

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
				MPLS:        mpls,
			})
		}
	}

	return rows
}

func hopKey(ip, host string) string {
	ip = strings.TrimSpace(ip)
	host = strings.TrimSpace(host)
	if ip != "" {
		return "ip:" + ip
	}
	if host != "" {
		return "host:" + strings.ToLower(host)
	}
	return "unknown"
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

func sortedSet(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	list := make([]string, 0, len(set))
	for k := range set {
		list = append(list, k)
	}
	sort.Strings(list)
	return list
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
