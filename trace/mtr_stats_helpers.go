package trace

import (
	"math"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

func (agg *MTRAggregator) ensureSlotLocked(ttl int) int {
	slot := ttl - 1
	if slot < len(agg.buckets) {
		return slot
	}
	need := slot + 1 - len(agg.buckets)
	agg.buckets = append(agg.buckets, make([]*mtrTTLBucket, need)...)
	agg.owned = append(agg.owned, make([]bool, need)...)
	agg.bucketSnapshots = append(agg.bucketSnapshots, make([]mtrBucketSnapshot, need)...)
	return slot
}

func (agg *MTRAggregator) writableBucketLocked(ttl int) *mtrTTLBucket {
	slot := agg.ensureSlotLocked(ttl)
	bucket := agg.buckets[slot]
	if bucket == nil {
		bucket = &mtrTTLBucket{index: make(map[mtrHopIdentity]int)}
		agg.buckets[slot] = bucket
		agg.owned[slot] = true
		return bucket
	}
	if !agg.owned[slot] {
		bucket = cloneMTRBucket(bucket)
		agg.buckets[slot] = bucket
		agg.owned[slot] = true
	}
	return bucket
}

func cloneMTRBucket(src *mtrTTLBucket) *mtrTTLBucket {
	dst := &mtrTTLBucket{
		rows:    make([]mtrHopAccum, len(src.rows)),
		index:   make(map[mtrHopIdentity]int, len(src.index)),
		version: src.version,
	}
	for i, acc := range src.rows {
		dst.rows[i] = cloneMTRHopAccum(acc)
	}
	for identity, index := range src.index {
		dst.index[identity] = index
	}
	return dst
}

func cloneMTRHopAccum(src mtrHopAccum) mtrHopAccum {
	src.mplsSet = cloneMTRLabelSet(src.mplsSet)
	return src
}

func (agg *MTRAggregator) markBucketDirtyLocked(ttl int, bucket *mtrTTLBucket) {
	bucket.version++
	agg.bucketSnapshots[ttl-1] = mtrBucketSnapshot{}
}

func (agg *MTRAggregator) markDirtyLocked() {
	agg.version++
	agg.snapshotValid = false
}

func (agg *MTRAggregator) trimTrailingSlotsLocked() {
	length := len(agg.buckets)
	for length > 0 && agg.buckets[length-1] == nil {
		length--
	}
	agg.buckets = agg.buckets[:length]
	agg.owned = agg.owned[:length]
	agg.bucketSnapshots = agg.bucketSnapshots[:length]
}

func (agg *MTRAggregator) includeAttemptLocked(bucket *mtrTTLBucket, attempt Hop) {
	identity, host, ip := mtrHopIdentityForAttempt(attempt)
	index, ok := bucket.index[identity]
	if !ok {
		index = len(bucket.rows)
		bucket.index[identity] = index
		bucket.rows = append(bucket.rows, mtrHopAccum{
			identity: identity,
			best:     math.MaxFloat64,
			order:    agg.nextOrder,
		})
		agg.nextOrder++
	}
	acc := &bucket.rows[index]
	if acc.seenUpdate != agg.updateSeq {
		acc.seenUpdate = agg.updateSeq
		if ip != "" {
			acc.ip = ip
		}
		if host != "" {
			acc.host = host
		}
	}
	if attempt.Geo != nil && acc.geoUpdate != agg.updateSeq {
		acc.geo = cloneMTRGeo(attempt.Geo)
		acc.geoUpdate = agg.updateSeq
	}
	acc.sent++
	mergeMTRLabels(&acc.mplsSet, attempt.MPLS)
	if !attempt.Success {
		return
	}

	rttMs := float64(attempt.RTT) / float64(time.Millisecond)
	updateMTRVariability(acc, rttMs)
	acc.sum += rttMs
	acc.sumSq += rttMs * rttMs
	acc.received++
	acc.last = rttMs
	if rttMs > acc.worst {
		acc.worst = rttMs
	}
	if rttMs >= 0 && rttMs < acc.best {
		acc.best = rttMs
	}
}

func mtrHopIdentityForAttempt(attempt Hop) (mtrHopIdentity, string, string) {
	host := strings.TrimSpace(attempt.Hostname)
	if ip := mtrAddrToIP(attempt.Address); ip != nil {
		if addr, ok := netip.AddrFromSlice(ip); ok {
			addr = addr.Unmap()
			return mtrHopIdentity{kind: mtrHopIP, addr: addr}, host, addr.String()
		}
	}
	if attempt.Address != nil {
		network := strings.TrimSpace(attempt.Address.Network())
		value := strings.TrimSpace(attempt.Address.String())
		return mtrHopIdentity{kind: mtrHopFallback, network: network, value: value}, host, value
	}
	if host != "" {
		return mtrHopIdentity{kind: mtrHopHost, value: strings.ToLower(host)}, host, ""
	}
	return mtrHopIdentity{kind: mtrHopUnknown}, "", ""
}

func parseMTRAddr(value string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func mergeMTRHopAccum(dst, src *mtrHopAccum) {
	mergeMTRVariability(dst, src)
	dst.sent += src.sent
	dst.received += src.received
	if src.received > 0 {
		dst.sum += src.sum
		dst.sumSq += src.sumSq
		dst.last = src.last
		if src.best >= 0 && src.best < dst.best {
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
	mergeMTRLabelSet(&dst.mplsSet, src.mplsSet)
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

func mergeMTRLabelSet(dst *map[string]struct{}, src map[string]struct{}) {
	if len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[string]struct{}, len(src))
	}
	for label := range src {
		(*dst)[label] = struct{}{}
	}
}

func cloneMTRLabelSet(src map[string]struct{}) map[string]struct{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]struct{}, len(src))
	for label := range src {
		dst[label] = struct{}{}
	}
	return dst
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

	acc.logSum *= ratio
	acc.jitterSum *= ratio
	acc.sum = sumNew
	acc.sumSq = sumSqNew
	acc.received = acc.sent
}

func buildMTRHopStat(acc *mtrHopAccum, ttl int) MTRHopStat {
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
		Dropped:            lossCount,
		GMean:              mtrGeometricMean(acc),
		Jitter:             acc.jitter,
		JitterAvg:          mtrJitterMean(acc),
		JitterMax:          acc.jitterMax,
		JitterInterarrival: acc.jitterInterarrival,
		TTL:                ttl,
		Host:               acc.host,
		IP:                 acc.ip,
		Loss:               lossPct,
		Snt:                acc.sent,
		Last:               acc.last,
		Avg:                avg,
		Best:               best,
		Wrst:               acc.worst,
		StDev:              stdev,
		Geo:                acc.geo,
		MPLS:               mpls,
		Received:           acc.received,
	}
}

func cloneMTRStats(src []MTRHopStat) []MTRHopStat {
	if src == nil {
		return nil
	}
	dst := make([]MTRHopStat, len(src))
	for i, stat := range src {
		dst[i] = stat
		dst[i].Geo = cloneMTRGeo(stat.Geo)
		if stat.MPLS != nil {
			dst[i].MPLS = make([]string, len(stat.MPLS))
			copy(dst[i].MPLS, stat.MPLS)
		}
		dst[i].Response = cloneMTRProbeResponse(stat.Response)
	}
	return dst
}

func cloneMTRGeo(src *ipgeo.IPGeoData) *ipgeo.IPGeoData {
	if src == nil {
		return nil
	}
	dst := *src
	if src.Router != nil {
		dst.Router = make(map[string][]string, len(src.Router))
		for asn, route := range src.Router {
			if route == nil {
				dst.Router[asn] = nil
				continue
			}
			dst.Router[asn] = make([]string, len(route))
			copy(dst.Router[asn], route)
		}
	}
	return &dst
}

// mergeUnknownIntoSingleKnown folds timeouts into the only observed path at a
// TTL. With zero or multiple known paths, the unknown row stays independent.
func mergeUnknownIntoSingleKnown(bucket *mtrTTLBucket) {
	unknownIdentity := mtrHopIdentity{kind: mtrHopUnknown}
	unknownIndex, ok := bucket.index[unknownIdentity]
	if !ok || len(bucket.rows) != 2 {
		return
	}
	knownIndex := 1 - unknownIndex
	mergeMTRHopAccum(&bucket.rows[knownIndex], &bucket.rows[unknownIndex])
	bucket.rows = append(bucket.rows[:unknownIndex], bucket.rows[unknownIndex+1:]...)
	rebuildMTRBucketIndex(bucket)
}

func rebuildMTRBucketIndex(bucket *mtrTTLBucket) {
	clear(bucket.index)
	for i := range bucket.rows {
		bucket.index[bucket.rows[i].identity] = i
	}
}

func mtrBucketNeedsMetadataPatch(
	bucket *mtrTTLBucket,
	ip string,
	targetAddr netip.Addr,
	hasTargetAddr bool,
	host string,
	geo *ipgeo.IPGeoData,
) bool {
	if bucket == nil {
		return false
	}
	for i := range bucket.rows {
		acc := &bucket.rows[i]
		if mtrAccumMatchesIP(acc, ip, targetAddr, hasTargetAddr) &&
			((host != "" && acc.host == "") || (geo != nil && acc.geo == nil)) {
			return true
		}
	}
	return false
}

func mtrAccumMatchesIP(acc *mtrHopAccum, ip string, targetAddr netip.Addr, hasTargetAddr bool) bool {
	if hasTargetAddr && acc.identity.kind == mtrHopIP {
		return acc.identity.addr == targetAddr
	}
	return strings.TrimSpace(acc.ip) == ip
}
