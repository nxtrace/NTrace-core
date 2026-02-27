package trace

import (
	"math"
	"sort"
	"strings"
	"sync"
	"time"

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
			accMap = make(map[string]*mtrHopAccum)
			agg.stats[ttl] = accMap
		}

		// 按 IP/Host 分组
		type groupData struct {
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
		groups := make(map[string]*groupData)

		for _, attempt := range attempts {
			host := strings.TrimSpace(attempt.Hostname)
			var ip string
			if attempt.Address != nil {
				ip = strings.TrimSpace(attempt.Address.String())
			}
			key := mtrHopKey(ip, host)
			g := groups[key]
			if g == nil {
				g = &groupData{
					host: host,
					ip:   ip,
					best: math.MaxFloat64,
				}
				groups[key] = g
			}
			g.count++
			if g.geo == nil && attempt.Geo != nil {
				g.geo = attempt.Geo
			}
			if len(attempt.MPLS) > 0 {
				if g.mpls == nil {
					g.mpls = make(map[string]struct{})
				}
				for _, label := range attempt.MPLS {
					val := strings.TrimSpace(label)
					if val != "" {
						g.mpls[val] = struct{}{}
					}
				}
			}
			if attempt.Success {
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
		}

		for key, g := range groups {
			acc := accMap[key]
			if acc == nil {
				acc = &mtrHopAccum{
					ttl:     ttl,
					key:     key,
					best:    math.MaxFloat64,
					order:   agg.nextOrder,
					mplsSet: make(map[string]struct{}),
				}
				agg.nextOrder++
				accMap[key] = acc
			}
			if g.ip != "" {
				acc.ip = g.ip
			}
			if g.host != "" {
				acc.host = g.host
			}
			if g.geo != nil {
				acc.geo = g.geo
			}
			acc.sent += g.count
			if g.received > 0 {
				acc.sum += g.sum
				acc.sumSq += g.sumSq
				acc.received += g.received
				acc.last = g.last
				if g.best > 0 && (acc.best == math.MaxFloat64 || g.best < acc.best) {
					acc.best = g.best
				}
				if g.worst > acc.worst {
					acc.worst = g.worst
				}
			}
			for label := range g.mpls {
				acc.mplsSet[label] = struct{}{}
			}
		}
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
				// 样本标准差: sqrt( (ΣX² - (ΣX)²/n) / (n-1) )
				n := float64(acc.received)
				variance := (acc.sumSq - (acc.sum*acc.sum)/n) / (n - 1)
				if variance > 0 {
					stdev = math.Sqrt(variance)
				}
			}

			var mpls []string
			if len(acc.mplsSet) > 0 {
				mpls = make([]string, 0, len(acc.mplsSet))
				for k := range acc.mplsSet {
					mpls = append(mpls, k)
				}
				sort.Strings(mpls)
			}

			rows = append(rows, MTRHopStat{
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
			})
		}
	}
	return rows
}

func mtrHopKey(ip, host string) string {
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
