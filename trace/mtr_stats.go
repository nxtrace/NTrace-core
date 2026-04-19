package trace

import (
	"sort"
	"strings"
	"sync"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

// ---------------------------------------------------------------------------
// MTR 聚合统计模型（公共层，CLI 和 Server 均可使用）
// ---------------------------------------------------------------------------

// MTRHopStat 表示 MTR 输出中一行统计数据。
type MTRHopStat struct {
	TTL      int              `json:"ttl"`
	Host     string           `json:"host,omitempty"`
	IP       string           `json:"ip,omitempty"`
	Loss     float64          `json:"loss_percent"`
	Snt      int              `json:"snt"`
	Last     float64          `json:"last_ms"`
	Avg      float64          `json:"avg_ms"`
	Best     float64          `json:"best_ms"`
	Wrst     float64          `json:"wrst_ms"`
	StDev    float64          `json:"stdev_ms"`
	Geo      *ipgeo.IPGeoData `json:"geo,omitempty"`
	MPLS     []string         `json:"mpls,omitempty"`
	Received int              `json:"received"`
}

// MTRSnapshot 是某一时刻的完整快照。
type MTRSnapshot struct {
	Iteration int          `json:"iteration"`
	Stats     []MTRHopStat `json:"stats"`
}

// ---------------------------------------------------------------------------
// 内部累加器
// ---------------------------------------------------------------------------

type mtrHopAccum struct {
	ttl      int
	key      string
	host     string
	ip       string
	sent     int
	received int
	sum      float64
	sumSq    float64 // Σ(rtt²)，用于在线方差
	last     float64
	best     float64
	worst    float64
	geo      *ipgeo.IPGeoData
	order    int
	mplsSet  map[string]struct{}
}

// MTRAggregator 跨轮次聚合 hop 统计。线程安全。
type MTRAggregator struct {
	mu        sync.Mutex
	stats     map[int]map[string]*mtrHopAccum // [ttl][key]
	nextOrder int
}

// NewMTRAggregator 创建新的聚合器。
func NewMTRAggregator() *MTRAggregator {
	return &MTRAggregator{
		stats: make(map[int]map[string]*mtrHopAccum),
	}
}

// Update 接收一轮 traceroute 的 Result 并更新统计，返回当前快照。
func (agg *MTRAggregator) Update(res *Result, queries int) []MTRHopStat {
	agg.mu.Lock()
	defer agg.mu.Unlock()

	if res == nil || len(res.Hops) == 0 {
		return agg.snapshotLocked()
	}

	_ = queries

	for idx, attempts := range res.Hops {
		if len(attempts) == 0 {
			continue
		}
		ttl := idx + 1
		accMap := agg.accMapForTTLLocked(ttl)
		for key, group := range groupMTRHopAttempts(attempts) {
			agg.mergeGroupedHopLocked(ttl, accMap, key, group)
		}
		mergeUnknownIntoSingleKnown(accMap)
	}

	return agg.snapshotLocked()
}

// Reset 清空所有统计数据，用于 r 键重置。
func (agg *MTRAggregator) Reset() {
	agg.mu.Lock()
	defer agg.mu.Unlock()
	agg.stats = make(map[int]map[string]*mtrHopAccum)
	agg.nextOrder = 0
}

// ClearHop 删除指定 TTL 上的所有聚合数据。
// 用于 per-hop 调度器中 knownFinalTTL 下调时，擦除旧 finalTTL 的过期统计，
// 避免 ghost row，同时不会把旧 final 的 Snt 合并到新 final（防止 Snt 膨胀）。
func (agg *MTRAggregator) ClearHop(ttl int) {
	agg.mu.Lock()
	defer agg.mu.Unlock()
	delete(agg.stats, ttl)
}

// MigrateStats 将 fromTTL 上所有累加器迁移合并到 toTTL，然后删除 fromTTL。
// 用于 knownFinalTTL 下调时把旧 finalTTL 上已入账的 dst-ip 统计搬到新 finalTTL。
// maxPerHop > 0 时，合并后对每个累加器的 sent/received 做上限裁剪，
// 保证 Snt 不超过预算。
func (agg *MTRAggregator) MigrateStats(fromTTL, toTTL, maxPerHop int) {
	agg.mu.Lock()
	defer agg.mu.Unlock()

	fromMap := agg.stats[fromTTL]
	if len(fromMap) == 0 {
		return
	}

	toMap := agg.accMapForTTLLocked(toTTL)

	for key, src := range fromMap {
		dst := toMap[key]
		if dst == nil {
			src.ttl = toTTL
			toMap[key] = src
			continue
		}
		mergeMTRHopAccum(dst, src)
	}

	for _, acc := range toMap {
		capMTRHopAccum(acc, maxPerHop)
	}

	delete(agg.stats, fromTTL)
}

// Clone 返回深拷贝的聚合器，用于流式预览（不影响原始数据）。
func (agg *MTRAggregator) Clone() *MTRAggregator {
	agg.mu.Lock()
	defer agg.mu.Unlock()

	c := &MTRAggregator{
		stats:     make(map[int]map[string]*mtrHopAccum, len(agg.stats)),
		nextOrder: agg.nextOrder,
	}
	for ttl, accMap := range agg.stats {
		cMap := make(map[string]*mtrHopAccum, len(accMap))
		for key, acc := range accMap {
			dup := *acc // 浅拷贝
			dup.mplsSet = make(map[string]struct{}, len(acc.mplsSet))
			for k := range acc.mplsSet {
				dup.mplsSet[k] = struct{}{}
			}
			if acc.geo != nil {
				geoCopy := *acc.geo
				dup.geo = &geoCopy
			}
			cMap[key] = &dup
		}
		c.stats[ttl] = cMap
	}
	return c
}

// Snapshot 返回当前聚合结果快照。
func (agg *MTRAggregator) Snapshot() []MTRHopStat {
	agg.mu.Lock()
	defer agg.mu.Unlock()
	return agg.snapshotLocked()
}

// PatchMetadataByIP updates existing rows for the given IP with late-arriving
// host/geo data without affecting sent/received/RTT statistics.
func (agg *MTRAggregator) PatchMetadataByIP(ip, host string, geo *ipgeo.IPGeoData) bool {
	agg.mu.Lock()
	defer agg.mu.Unlock()

	ip = strings.TrimSpace(ip)
	host = strings.TrimSpace(host)
	if ip == "" || (host == "" && geo == nil) {
		return false
	}

	changed := false
	for _, accMap := range agg.stats {
		for _, acc := range accMap {
			if strings.TrimSpace(acc.ip) != ip {
				continue
			}
			if host != "" && acc.host == "" {
				acc.host = host
				changed = true
			}
			if geo != nil && acc.geo == nil {
				geoCopy := *geo
				acc.geo = &geoCopy
				changed = true
			}
		}
	}
	return changed
}

func (agg *MTRAggregator) snapshotLocked() []MTRHopStat {
	// 收集 TTL 列表并排序
	ttls := make([]int, 0, len(agg.stats))
	for ttl := range agg.stats {
		ttls = append(ttls, ttl)
	}
	sort.Ints(ttls)

	var rows []MTRHopStat
	for _, ttl := range ttls {
		accMap := agg.stats[ttl]
		if len(accMap) == 0 {
			continue
		}

		// 按 order 稳定排序
		accs := make([]*mtrHopAccum, 0, len(accMap))
		for _, acc := range accMap {
			accs = append(accs, acc)
		}
		sort.SliceStable(accs, func(i, j int) bool {
			if accs[i].order == accs[j].order {
				return accs[i].ip < accs[j].ip
			}
			return accs[i].order < accs[j].order
		})

		for _, acc := range accs {
			rows = append(rows, buildMTRHopStat(acc))
		}
	}
	return rows
}

// mtrUnknownKey 是 timeout / 无地址 hop 的聚合键。
const mtrUnknownKey = "unknown"

func mtrHopKey(ip, host string) string {
	ip = strings.TrimSpace(ip)
	host = strings.TrimSpace(host)
	if ip != "" {
		return "ip:" + ip
	}
	if host != "" {
		return "host:" + strings.ToLower(host)
	}
	return mtrUnknownKey
}

// mergeUnknownIntoSingleKnown 在同一 TTL 的 accMap 中，
// 如果恰好只有 1 条非 unknown 路径，则将 unknown 累加器归并到该路径，
// 避免同一跳同时出现 "(waiting for reply)" 和真实 IP 两行。
//
// 多路径场景（非 unknown ≥ 2 或 == 0）不归并，防止误归因。
func mergeUnknownIntoSingleKnown(accMap map[string]*mtrHopAccum) {
	unk, ok := accMap[mtrUnknownKey]
	if !ok {
		return
	}

	// 收集非 unknown 累加器
	var known *mtrHopAccum
	knownCount := 0
	for k, acc := range accMap {
		if k == mtrUnknownKey {
			continue
		}
		known = acc
		knownCount++
		if knownCount > 1 {
			break // 多路径，不归并
		}
	}
	if knownCount != 1 || known == nil {
		return
	}

	mergeMTRHopAccum(known, unk)
	delete(accMap, mtrUnknownKey)
}
