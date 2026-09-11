package trace

import (
	"net/netip"
	"sort"
	"strings"
	"sync"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

// MTRHopStat 表示 MTR 输出中一行统计数据。
type MTRHopStat struct {
	Dropped            int               `json:"dropped" jsonschema:"Completed probes without a reply on this responder row"`
	GMean              float64           `json:"gmean_ms" jsonschema:"Geometric mean of successful RTTs in milliseconds"`
	Jitter             float64           `json:"jitter_ms" jsonschema:"Absolute difference between successive successful RTTs in milliseconds"`
	JitterAvg          float64           `json:"jitter_avg_ms" jsonschema:"Mean jitter including the first zero sample in milliseconds"`
	JitterMax          float64           `json:"jitter_max_ms" jsonschema:"Maximum jitter in milliseconds"`
	JitterInterarrival float64           `json:"jitter_interarrival_ms" jsonschema:"mtr-scale interarrival accumulator in milliseconds: I = 15/16 I + jitter, not the divided RFC estimate"`
	TTL                int               `json:"ttl"`
	Host               string            `json:"host,omitempty"`
	IP                 string            `json:"ip,omitempty"`
	Loss               float64           `json:"loss_percent"`
	Snt                int               `json:"snt"`
	Last               float64           `json:"last_ms"`
	Avg                float64           `json:"avg_ms"`
	Best               float64           `json:"best_ms"`
	Wrst               float64           `json:"wrst_ms"`
	StDev              float64           `json:"stdev_ms"`
	Geo                *ipgeo.IPGeoData  `json:"geo,omitempty"`
	MPLS               []string          `json:"mpls,omitempty"`
	Received           int               `json:"received"`
	Response           *MTRProbeResponse `json:"response,omitempty"`
}

// MTRSnapshot 是某一时刻的完整快照。
type MTRSnapshot struct {
	Iteration int          `json:"iteration"`
	Stats     []MTRHopStat `json:"stats"`
}

type mtrHopIdentityKind uint8

const (
	mtrHopUnknown mtrHopIdentityKind = iota
	mtrHopIP
	mtrHopHost
	mtrHopFallback
)

// mtrHopIdentity contains only comparable fields. Unmapped addresses make
// ordinary and IPv4-mapped observations share one row.
type mtrHopIdentity struct {
	kind    mtrHopIdentityKind
	addr    netip.Addr
	network string
	value   string
}

type mtrHopAccum struct {
	identity           mtrHopIdentity
	host               string
	ip                 string
	sent               int
	received           int
	sum                float64
	sumSq              float64 // Σ(rtt²)，用于在线方差
	logSum             float64
	hasZeroRTT         bool
	first              float64
	jitter             float64
	jitterSum          float64
	jitterMax          float64
	jitterInterarrival float64
	last               float64
	best               float64
	worst              float64
	geo                *ipgeo.IPGeoData
	order              uint64
	mplsSet            map[string]struct{}
	seenUpdate         uint64
	geoUpdate          uint64
}

// mtrTTLBucket keeps rows in first-observation order. index points into rows,
// so growing the slice never invalidates an identity lookup.
type mtrTTLBucket struct {
	rows    []mtrHopAccum
	index   map[mtrHopIdentity]int
	version uint64
}

// Snapshots belong to an aggregator rather than a potentially shared bucket,
// allowing cloned aggregators to snapshot concurrently without shared writes.
type mtrBucketSnapshot struct {
	bucket  *mtrTTLBucket
	version uint64
	rows    []MTRHopStat
}

// MTRAggregator 跨轮次聚合 hop 统计。线程安全。
type MTRAggregator struct {
	mu sync.Mutex

	buckets         []*mtrTTLBucket // index = TTL - 1
	owned           []bool          // false means copy before mutation
	bucketSnapshots []mtrBucketSnapshot

	nextOrder uint64
	updateSeq uint64
	version   uint64

	snapshotVersion uint64
	snapshotValid   bool
	snapshot        []MTRHopStat
}

// NewMTRAggregator 创建新的聚合器。
func NewMTRAggregator() *MTRAggregator {
	return &MTRAggregator{}
}

// Update 接收一轮 traceroute 的 Result 并更新统计，返回当前快照。
func (agg *MTRAggregator) Update(res *Result, queries int) []MTRHopStat {
	agg.mu.Lock()
	defer agg.mu.Unlock()

	if res == nil || len(res.Hops) == 0 {
		return agg.snapshotLocked()
	}

	_ = queries
	agg.updateSeq++
	mutated := false
	for idx, attempts := range res.Hops {
		if len(attempts) == 0 {
			continue
		}

		ttl := idx + 1
		bucket := agg.writableBucketLocked(ttl)
		for _, attempt := range attempts {
			agg.includeAttemptLocked(bucket, attempt)
		}
		mergeUnknownIntoSingleKnown(bucket)
		agg.markBucketDirtyLocked(ttl, bucket)
		mutated = true
	}
	if mutated {
		agg.markDirtyLocked()
	}

	return agg.snapshotLocked()
}

// Reset 清空所有统计数据，用于 r 键重置。
func (agg *MTRAggregator) Reset() {
	agg.mu.Lock()
	defer agg.mu.Unlock()

	clear(agg.buckets)
	agg.buckets = agg.buckets[:0]
	clear(agg.owned)
	agg.owned = agg.owned[:0]
	clear(agg.bucketSnapshots)
	agg.bucketSnapshots = agg.bucketSnapshots[:0]
	agg.snapshot = nil
	agg.nextOrder = 0
	agg.updateSeq = 0
	agg.markDirtyLocked()
}

// ClearHop 删除指定 TTL 上的所有聚合数据。
func (agg *MTRAggregator) ClearHop(ttl int) {
	agg.mu.Lock()
	defer agg.mu.Unlock()

	slot := ttl - 1
	if slot < 0 || slot >= len(agg.buckets) || agg.buckets[slot] == nil {
		return
	}
	agg.buckets[slot] = nil
	agg.owned[slot] = false
	agg.bucketSnapshots[slot] = mtrBucketSnapshot{}
	agg.trimTrailingSlotsLocked()
	agg.markDirtyLocked()
}

// ClearAbove removes statistics beyond a confirmed sticky destination edge.
func (agg *MTRAggregator) ClearAbove(ttl int) {
	agg.mu.Lock()
	defer agg.mu.Unlock()

	start := max(ttl, 0)
	if start >= len(agg.buckets) {
		return
	}
	changed := false
	for slot := start; slot < len(agg.buckets); slot++ {
		if agg.buckets[slot] != nil {
			changed = true
		}
		agg.buckets[slot] = nil
		agg.owned[slot] = false
		agg.bucketSnapshots[slot] = mtrBucketSnapshot{}
	}
	agg.trimTrailingSlotsLocked()
	if changed {
		agg.markDirtyLocked()
	}
}

func filterMTRStatsAtPathEnd(stats []MTRHopStat, ttl int) []MTRHopStat {
	filtered := stats[:0]
	for _, stat := range stats {
		if stat.TTL <= ttl {
			filtered = append(filtered, stat)
		}
	}
	return filtered
}

// MigrateStats 将 fromTTL 上所有累加器迁移合并到 toTTL，然后删除 fromTTL。
func (agg *MTRAggregator) MigrateStats(fromTTL, toTTL, maxPerHop int) {
	agg.mu.Lock()
	defer agg.mu.Unlock()

	if fromTTL <= 0 || toTTL <= 0 || fromTTL == toTTL {
		return
	}
	fromSlot := fromTTL - 1
	if fromSlot >= len(agg.buckets) || agg.buckets[fromSlot] == nil {
		return
	}

	agg.ensureSlotLocked(toTTL)
	toSlot := toTTL - 1
	from := agg.buckets[fromSlot]
	if agg.buckets[toSlot] == nil {
		agg.buckets[toSlot] = from
		agg.owned[toSlot] = agg.owned[fromSlot]
		agg.buckets[fromSlot] = nil
		agg.owned[fromSlot] = false
		agg.bucketSnapshots[fromSlot] = mtrBucketSnapshot{}
		agg.bucketSnapshots[toSlot] = mtrBucketSnapshot{}

		if maxPerHop > 0 {
			to := agg.writableBucketLocked(toTTL)
			for i := range to.rows {
				capMTRHopAccum(&to.rows[i], maxPerHop)
			}
			agg.markBucketDirtyLocked(toTTL, to)
		}
	} else {
		to := agg.writableBucketLocked(toTTL)
		for i := range from.rows {
			src := &from.rows[i]
			if dstIndex, ok := to.index[src.identity]; ok {
				mergeMTRHopAccum(&to.rows[dstIndex], src)
				continue
			}
			to.index[src.identity] = len(to.rows)
			to.rows = append(to.rows, cloneMTRHopAccum(*src))
		}
		sort.SliceStable(to.rows, func(i, j int) bool {
			return to.rows[i].order < to.rows[j].order
		})
		rebuildMTRBucketIndex(to)
		for i := range to.rows {
			capMTRHopAccum(&to.rows[i], maxPerHop)
		}
		agg.markBucketDirtyLocked(toTTL, to)

		agg.buckets[fromSlot] = nil
		agg.owned[fromSlot] = false
		agg.bucketSnapshots[fromSlot] = mtrBucketSnapshot{}
	}

	agg.trimTrailingSlotsLocked()
	agg.markDirtyLocked()
}

// Clone returns a copy-on-write aggregator for streaming previews. Only a TTL
// changed through the clone gets a private bucket.
func (agg *MTRAggregator) Clone() *MTRAggregator {
	agg.mu.Lock()
	defer agg.mu.Unlock()

	c := &MTRAggregator{
		buckets:         append([]*mtrTTLBucket(nil), agg.buckets...),
		owned:           make([]bool, len(agg.buckets)),
		bucketSnapshots: append([]mtrBucketSnapshot(nil), agg.bucketSnapshots...),
		nextOrder:       agg.nextOrder,
		updateSeq:       agg.updateSeq,
		version:         agg.version,
		snapshotVersion: agg.snapshotVersion,
		snapshotValid:   agg.snapshotValid,
		snapshot:        agg.snapshot,
	}
	for slot, bucket := range agg.buckets {
		if bucket != nil {
			agg.owned[slot] = false
		}
	}
	return c
}

// Snapshot 返回当前聚合结果快照。
func (agg *MTRAggregator) Snapshot() []MTRHopStat {
	agg.mu.Lock()
	defer agg.mu.Unlock()
	return agg.snapshotLocked()
}

// PatchMetadataByIP updates existing rows without changing statistics or row order.
func (agg *MTRAggregator) PatchMetadataByIP(ip, host string, geo *ipgeo.IPGeoData) bool {
	agg.mu.Lock()
	defer agg.mu.Unlock()

	ip = strings.TrimSpace(ip)
	host = strings.TrimSpace(host)
	if ip == "" || (host == "" && geo == nil) {
		return false
	}
	targetAddr, hasTargetAddr := parseMTRAddr(ip)

	changed := false
	var geoCopy *ipgeo.IPGeoData
	for slot, readBucket := range agg.buckets {
		if !mtrBucketNeedsMetadataPatch(readBucket, ip, targetAddr, hasTargetAddr, host, geo) {
			continue
		}
		bucket := agg.writableBucketLocked(slot + 1)
		bucketChanged := false
		for i := range bucket.rows {
			acc := &bucket.rows[i]
			if !mtrAccumMatchesIP(acc, ip, targetAddr, hasTargetAddr) {
				continue
			}
			if host != "" && acc.host == "" {
				acc.host = host
				bucketChanged = true
			}
			if geo != nil && acc.geo == nil {
				if geoCopy == nil {
					geoCopy = cloneMTRGeo(geo)
				}
				acc.geo = geoCopy
				bucketChanged = true
			}
		}
		if bucketChanged {
			agg.markBucketDirtyLocked(slot+1, bucket)
			changed = true
		}
	}
	if changed {
		agg.markDirtyLocked()
	}
	return changed
}

func (agg *MTRAggregator) snapshotLocked() []MTRHopStat {
	if !agg.snapshotValid || agg.snapshotVersion != agg.version {
		rowCount := 0
		for _, bucket := range agg.buckets {
			if bucket != nil {
				rowCount += len(bucket.rows)
			}
		}
		var rows []MTRHopStat
		if rowCount > 0 {
			rows = make([]MTRHopStat, 0, rowCount)
		}
		for slot, bucket := range agg.buckets {
			if bucket == nil || len(bucket.rows) == 0 {
				continue
			}
			cached := &agg.bucketSnapshots[slot]
			if cached.bucket != bucket || cached.version != bucket.version {
				bucketRows := make([]MTRHopStat, len(bucket.rows))
				for i := range bucket.rows {
					bucketRows[i] = buildMTRHopStat(&bucket.rows[i], slot+1)
				}
				*cached = mtrBucketSnapshot{bucket: bucket, version: bucket.version, rows: bucketRows}
			}
			rows = append(rows, cached.rows...)
		}
		agg.snapshot = rows
		agg.snapshotVersion = agg.version
		agg.snapshotValid = true
	}
	return cloneMTRStats(agg.snapshot)
}
