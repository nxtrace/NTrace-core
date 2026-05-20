package printer

import (
	"sort"
	"sync"
	"time"

	"github.com/nxtrace/NTrace-core/trace"
)

const MTRHistoryWindow = 3 * time.Minute

type MTRHistoryChartMode int

const (
	MTRHistoryHeatmap MTRHistoryChartMode = iota
	MTRHistoryBars
	MTRHistorySparkline
)

type MTRHistorySample struct {
	At      time.Time
	RTT     time.Duration
	Timeout bool
}

type MTRHistoryTTL struct {
	TTL     int
	Samples []MTRHistorySample
}

type MTRHistoryStore struct {
	mu     sync.Mutex
	window time.Duration
	byTTL  map[int][]MTRHistorySample
}

func NewMTRHistoryStore(window time.Duration) *MTRHistoryStore {
	if window <= 0 {
		window = MTRHistoryWindow
	}
	return &MTRHistoryStore{
		window: window,
		byTTL:  make(map[int][]MTRHistorySample),
	}
}

func (s *MTRHistoryStore) AddProbeEvent(event trace.MTRProbeEvent) {
	if s == nil || event.TTL <= 0 {
		return
	}
	at := event.Timestamp
	if at.IsZero() {
		at = time.Now()
	}
	sample := MTRHistorySample{
		At:      at,
		RTT:     event.RTT,
		Timeout: !event.Success,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byTTL[event.TTL] = append(s.byTTL[event.TTL], sample)
	s.pruneLocked(at)
}

func (s *MTRHistoryStore) Reset() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.byTTL)
}

func (s *MTRHistoryStore) Snapshot(now time.Time) []MTRHistoryTTL {
	if s == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(now)

	ttls := make([]int, 0, len(s.byTTL))
	for ttl := range s.byTTL {
		ttls = append(ttls, ttl)
	}
	sort.Ints(ttls)

	out := make([]MTRHistoryTTL, 0, len(ttls))
	for _, ttl := range ttls {
		samples := s.byTTL[ttl]
		cp := make([]MTRHistorySample, len(samples))
		copy(cp, samples)
		out = append(out, MTRHistoryTTL{TTL: ttl, Samples: cp})
	}
	return out
}

func (s *MTRHistoryStore) pruneLocked(now time.Time) {
	cutoff := now.Add(-s.window)
	for ttl, samples := range s.byTTL {
		kept := samples[:0]
		for _, sample := range samples {
			if !sample.At.Before(cutoff) {
				kept = append(kept, sample)
			}
		}
		if len(kept) == 0 {
			delete(s.byTTL, ttl)
			continue
		}
		s.byTTL[ttl] = kept
	}
}
