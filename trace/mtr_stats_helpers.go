package trace

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

type mtrHopGroup struct {
	host     string
	ip       string
	geo      *ipgeo.IPGeoData
	sum      float64
	sumSq    float64
	last     float64
	best     float64
	worst    float64
	received int
	count    int
	mpls     map[string]struct{}
}

func newMTRHopGroup(host, ip string) *mtrHopGroup {
	return &mtrHopGroup{
		host: host,
		ip:   ip,
		best: math.MaxFloat64,
	}
}

func groupMTRHopAttempts(attempts []Hop) map[string]*mtrHopGroup {
	groups := make(map[string]*mtrHopGroup)
	for _, attempt := range attempts {
		host := strings.TrimSpace(attempt.Hostname)
		ip := strings.TrimSpace(mtrAddrString(attempt.Address))
		key := mtrHopKey(ip, host)
		group := groups[key]
		if group == nil {
			group = newMTRHopGroup(host, ip)
			groups[key] = group
		}
		group.includeAttempt(attempt)
	}
	return groups
}

func (g *mtrHopGroup) includeAttempt(attempt Hop) {
	g.count++
	if g.geo == nil && attempt.Geo != nil {
		g.geo = attempt.Geo
	}
	mergeMTRLabels(&g.mpls, attempt.MPLS)
	if !attempt.Success {
		return
	}

	rttMs := float64(attempt.RTT) / float64(time.Millisecond)
	g.sum += rttMs
	g.sumSq += rttMs * rttMs
	g.received++
	g.last = rttMs
	if rttMs > g.worst {
		g.worst = rttMs
	}
	if rttMs > 0 && rttMs < g.best {
		g.best = rttMs
	}
}

func (agg *MTRAggregator) accMapForTTLLocked(ttl int) map[string]*mtrHopAccum {
	accMap := agg.stats[ttl]
	if accMap == nil {
		accMap = make(map[string]*mtrHopAccum)
		agg.stats[ttl] = accMap
	}
	return accMap
}

func (agg *MTRAggregator) newHopAccum(ttl int, key string) *mtrHopAccum {
	acc := &mtrHopAccum{
		ttl:     ttl,
		key:     key,
		best:    math.MaxFloat64,
		order:   agg.nextOrder,
		mplsSet: make(map[string]struct{}),
	}
	agg.nextOrder++
	return acc
}

func (agg *MTRAggregator) mergeGroupedHopLocked(ttl int, accMap map[string]*mtrHopAccum, key string, group *mtrHopGroup) {
	acc := accMap[key]
	if acc == nil {
		acc = agg.newHopAccum(ttl, key)
		accMap[key] = acc
	}
	mergeMTRHopGroupIntoAccum(acc, group)
}

func mergeMTRHopGroupIntoAccum(acc *mtrHopAccum, group *mtrHopGroup) {
	if group.ip != "" {
		acc.ip = group.ip
	}
	if group.host != "" {
		acc.host = group.host
	}
	if group.geo != nil {
		acc.geo = group.geo
	}
	acc.sent += group.count
	if group.received > 0 {
		acc.sum += group.sum
		acc.sumSq += group.sumSq
		acc.received += group.received
		acc.last = group.last
		if group.best > 0 && (acc.best == math.MaxFloat64 || group.best < acc.best) {
			acc.best = group.best
		}
		if group.worst > acc.worst {
			acc.worst = group.worst
		}
	}
	mergeMTRLabelSet(acc.mplsSet, group.mpls)
}

func mergeMTRHopAccum(dst, src *mtrHopAccum) {
	dst.sent += src.sent
	dst.received += src.received
	if src.received > 0 {
		dst.sum += src.sum
		dst.sumSq += src.sumSq
		dst.last = src.last
		if src.best > 0 && src.best < dst.best {
			dst.best = src.best
		}
		if src.worst > dst.worst {
			dst.worst = src.worst
		}
	}
	if dst.geo == nil && src.geo != nil {
		dst.geo = src.geo
	}
	if dst.host == "" && src.host != "" {
		dst.host = src.host
	}
	if dst.ip == "" && src.ip != "" {
		dst.ip = src.ip
	}
	mergeMTRLabelSet(dst.mplsSet, src.mplsSet)
}

func mergeMTRLabels(dst *map[string]struct{}, labels []string) {
	if len(labels) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[string]struct{})
	}
	for _, label := range labels {
		val := strings.TrimSpace(label)
		if val != "" {
			(*dst)[val] = struct{}{}
		}
	}
}

func mergeMTRLabelSet(dst, src map[string]struct{}) {
	for label := range src {
		dst[label] = struct{}{}
	}
}

func capMTRHopAccum(acc *mtrHopAccum, maxPerHop int) {
	if maxPerHop <= 0 {
		return
	}
	if acc.sent > maxPerHop {
		acc.sent = maxPerHop
	}
	if acc.received <= acc.sent {
		return
	}

	nOrig := float64(acc.received)
	nNew := float64(acc.sent)
	ratio := nNew / nOrig
	sumNew := acc.sum * ratio
	ss := acc.sumSq - (acc.sum*acc.sum)/nOrig
	if ss < 0 {
		ss = 0
	}

	var sumSqNew float64
	if nOrig > 1 && nNew > 1 {
		sumSqNew = ss*(nNew-1)/(nOrig-1) + (sumNew*sumNew)/nNew
	} else {
		sumSqNew = (sumNew * sumNew) / nNew
	}

	acc.sum = sumNew
	acc.sumSq = sumSqNew
	acc.received = acc.sent
}

func buildMTRHopStat(acc *mtrHopAccum) MTRHopStat {
	lossCount := acc.sent - acc.received
	lossPct := 0.0
	if acc.sent > 0 {
		lossPct = float64(lossCount) / float64(acc.sent) * 100
	}

	best := acc.best
	if best == math.MaxFloat64 {
		best = 0
	}

	avg := 0.0
	if acc.received > 0 {
		avg = acc.sum / float64(acc.received)
	}

	stdev := 0.0
	if acc.received > 1 {
		n := float64(acc.received)
		variance := (acc.sumSq - (acc.sum*acc.sum)/n) / (n - 1)
		if variance > 0 {
			stdev = math.Sqrt(variance)
		}
	}

	var mpls []string
	if len(acc.mplsSet) > 0 {
		mpls = make([]string, 0, len(acc.mplsSet))
		for label := range acc.mplsSet {
			mpls = append(mpls, label)
		}
		sort.Strings(mpls)
	}

	return MTRHopStat{
		TTL:      acc.ttl,
		Host:     acc.host,
		IP:       acc.ip,
		Loss:     lossPct,
		Snt:      acc.sent,
		Last:     acc.last,
		Avg:      avg,
		Best:     best,
		Wrst:     acc.worst,
		StDev:    stdev,
		Geo:      acc.geo,
		MPLS:     mpls,
		Received: acc.received,
	}
}
