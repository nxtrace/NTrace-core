package trace

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

// ---------------------------------------------------------------------------
// Mock TTL prober for scheduler tests
// ---------------------------------------------------------------------------

type mockTTLProber struct {
	mu       sync.Mutex
	probeFn  func(ctx context.Context, ttl int) (mtrProbeResult, error)
	resetCnt int32
	closeCnt int32
	probeCnt int32
	probeLog []int // ttl of each probe call
}

func (m *mockTTLProber) ProbeTTL(ctx context.Context, ttl int) (mtrProbeResult, error) {
	atomic.AddInt32(&m.probeCnt, 1)
	m.mu.Lock()
	m.probeLog = append(m.probeLog, ttl)
	m.mu.Unlock()
	if m.probeFn != nil {
		return m.probeFn(ctx, ttl)
	}
	return mtrProbeResult{TTL: ttl}, nil
}

func (m *mockTTLProber) Reset() error {
	atomic.AddInt32(&m.resetCnt, 1)
	return nil
}

func (m *mockTTLProber) Close() error {
	atomic.AddInt32(&m.closeCnt, 1)
	return nil
}

func (m *mockTTLProber) getProbeCount() int {
	return int(atomic.LoadInt32(&m.probeCnt))
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestScheduler_MaxPerHopCompletion(t *testing.T) {
	dstIP := net.ParseIP("10.0.0.5")

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			// Simulate: TTL 3 is the destination
			if ttl == 3 {
				return mtrProbeResult{
					TTL:     ttl,
					Success: true,
					Addr:    &net.IPAddr{IP: dstIP},
					RTT:     10 * time.Millisecond,
				}, nil
			}
			return mtrProbeResult{
				TTL:     ttl,
				Success: true,
				Addr:    &net.IPAddr{IP: net.ParseIP("10.0.0." + string(rune('0'+ttl)))},
				RTT:     time.Duration(ttl) * time.Millisecond,
			}, nil
		},
	}

	agg := NewMTRAggregator()
	var lastIter int
	var snapshotCount int32

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          30,
		HopInterval:      time.Millisecond,
		MaxPerHop:        3,
		ParallelRequests: 5,
		ProgressThrottle: time.Millisecond,
		DstIP:            dstIP,
	}, func(iter int, stats []MTRHopStat) {
		atomic.AddInt32(&snapshotCount, 1)
		lastIter = iter
	}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should complete: each active TTL (1..3) should have 3 probes
	if lastIter != 3 {
		t.Errorf("expected final iteration=3, got %d", lastIter)
	}

	stats := agg.Snapshot()
	if len(stats) < 3 {
		t.Fatalf("expected at least 3 stats rows, got %d", len(stats))
	}

	for _, s := range stats {
		if s.TTL >= 1 && s.TTL <= 3 {
			if s.Snt != 3 {
				t.Errorf("TTL %d: expected Snt=3, got %d", s.TTL, s.Snt)
			}
		}
	}

	if atomic.LoadInt32(&prober.closeCnt) != 1 {
		t.Error("prober.Close() not called")
	}
}

func TestScheduler_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var probes int32
	prober := &mockTTLProber{
		probeFn: func(ctx context.Context, ttl int) (mtrProbeResult, error) {
			n := atomic.AddInt32(&probes, 1)
			if n >= 5 {
				cancel()
			}
			return mtrProbeResult{TTL: ttl}, nil
		},
	}

	agg := NewMTRAggregator()
	var snapshotCalled int32

	err := runMTRScheduler(ctx, prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          5,
		HopInterval:      time.Millisecond,
		MaxPerHop:        0, // unlimited
		ParallelRequests: 5,
		ProgressThrottle: time.Millisecond,
	}, func(_ int, _ []MTRHopStat) {
		atomic.AddInt32(&snapshotCalled, 1)
	}, nil)

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if atomic.LoadInt32(&prober.closeCnt) != 1 {
		t.Error("prober.Close() not called on cancel")
	}
}

func TestScheduler_DestinationDetection(t *testing.T) {
	dstIP := net.ParseIP("8.8.8.8")

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			if ttl >= 5 {
				return mtrProbeResult{
					TTL:     ttl,
					Success: true,
					Addr:    &net.IPAddr{IP: dstIP},
					RTT:     50 * time.Millisecond,
				}, nil
			}
			return mtrProbeResult{
				TTL:     ttl,
				Success: true,
				Addr:    &net.IPAddr{IP: net.ParseIP("10.0.0.1")},
				RTT:     10 * time.Millisecond,
			}, nil
		},
	}

	agg := NewMTRAggregator()

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          30,
		HopInterval:      time.Millisecond,
		MaxPerHop:        2,
		ParallelRequests: 1, // serialize to ensure dest detection before higher TTLs
		ProgressThrottle: time.Millisecond,
		DstIP:            dstIP,
	}, nil, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := agg.Snapshot()
	// TTL 5 is the destination; higher TTLs should be disabled after detection.
	// With parallelism=1, at most TTL 6 could sneak in before the result is
	// processed (tick vs result race), so we allow a small margin.
	maxTTL := 0
	for _, s := range stats {
		if s.TTL > maxTTL {
			maxTTL = s.TTL
		}
	}
	if maxTTL > 6 {
		t.Errorf("expected max TTL <= 6 (destination detected at 5), got %d", maxTTL)
	}
}

func TestScheduler_Reset(t *testing.T) {
	var probes int32
	var resetOnce int32

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			atomic.AddInt32(&probes, 1)
			return mtrProbeResult{
				TTL:     ttl,
				Success: true,
				Addr:    &net.IPAddr{IP: net.ParseIP("10.0.0.1")},
				RTT:     5 * time.Millisecond,
			}, nil
		},
	}

	agg := NewMTRAggregator()

	var snapshotIters []int
	var iterMu sync.Mutex

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          2,
		HopInterval:      time.Millisecond,
		MaxPerHop:        4,
		ParallelRequests: 2,
		ProgressThrottle: time.Millisecond,
		IsResetRequested: func() bool {
			// Trigger reset after some probes have been done
			p := atomic.LoadInt32(&probes)
			if p >= 4 && atomic.CompareAndSwapInt32(&resetOnce, 0, 1) {
				return true
			}
			return false
		},
	}, func(iter int, _ []MTRHopStat) {
		iterMu.Lock()
		snapshotIters = append(snapshotIters, iter)
		iterMu.Unlock()
	}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After reset, iteration should restart from 0→1
	// Final iteration should be 4 (maxPerHop=4)
	iterMu.Lock()
	defer iterMu.Unlock()

	if len(snapshotIters) == 0 {
		t.Fatal("expected at least one snapshot")
	}
	lastIter := snapshotIters[len(snapshotIters)-1]
	if lastIter != 4 {
		t.Errorf("expected last iteration=4, got %d", lastIter)
	}

	// prober.Reset should have been called once
	if atomic.LoadInt32(&prober.resetCnt) != 1 {
		t.Errorf("expected 1 Reset call, got %d", atomic.LoadInt32(&prober.resetCnt))
	}
}

func TestScheduler_Pause(t *testing.T) {
	var pauseFlag int32
	var probes int32

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			n := atomic.AddInt32(&probes, 1)
			if n == 2 {
				// Pause after 2 probes, resume after 50ms
				atomic.StoreInt32(&pauseFlag, 1)
				go func() {
					time.Sleep(50 * time.Millisecond)
					atomic.StoreInt32(&pauseFlag, 0)
				}()
			}
			if n >= 10 {
				cancel()
			}
			return mtrProbeResult{TTL: ttl}, nil
		},
	}

	agg := NewMTRAggregator()

	err := runMTRScheduler(ctx, prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          2,
		HopInterval:      time.Millisecond,
		MaxPerHop:        0, // unlimited, cancelled by ctx
		ParallelRequests: 2,
		ProgressThrottle: time.Millisecond,
		IsPaused:         func() bool { return atomic.LoadInt32(&pauseFlag) == 1 },
	}, nil, nil)

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	// Should have probed at least 2 (before pause) + some more after resume
	p := atomic.LoadInt32(&probes)
	if p < 4 {
		t.Errorf("expected at least 4 probes (across pause), got %d", p)
	}
}

func TestScheduler_IterationIsMinSnt(t *testing.T) {
	// TTL 1 responds quickly, TTL 2 responds slowly
	var ttl1Count, ttl2Count int32

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			if ttl == 1 {
				atomic.AddInt32(&ttl1Count, 1)
				return mtrProbeResult{
					TTL:     1,
					Success: true,
					Addr:    &net.IPAddr{IP: net.ParseIP("10.0.0.1")},
					RTT:     1 * time.Millisecond,
				}, nil
			}
			atomic.AddInt32(&ttl2Count, 1)
			time.Sleep(10 * time.Millisecond) // slower
			return mtrProbeResult{
				TTL:     2,
				Success: true,
				Addr:    &net.IPAddr{IP: net.ParseIP("10.0.0.2")},
				RTT:     10 * time.Millisecond,
			}, nil
		},
	}

	agg := NewMTRAggregator()
	var finalIter int

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          2,
		HopInterval:      time.Millisecond,
		MaxPerHop:        3,
		ParallelRequests: 2,
		ProgressThrottle: time.Millisecond,
	}, func(iter int, _ []MTRHopStat) {
		finalIter = iter
	}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both TTLs should have 3 probes (MaxPerHop=3)
	if finalIter != 3 {
		t.Errorf("expected final iteration=3, got %d", finalIter)
	}
}

func TestScheduler_OnProbeCallback(t *testing.T) {
	dstIP := net.ParseIP("10.0.0.3")

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			if ttl == 3 {
				return mtrProbeResult{
					TTL: ttl, Success: true,
					Addr: &net.IPAddr{IP: dstIP},
					RTT:  5 * time.Millisecond,
				}, nil
			}
			return mtrProbeResult{
				TTL: ttl, Success: true,
				Addr: &net.IPAddr{IP: net.ParseIP("10.0.0.1")},
				RTT:  1 * time.Millisecond,
			}, nil
		},
	}

	agg := NewMTRAggregator()

	var callbackResults []mtrProbeResult
	var mu sync.Mutex

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          30,
		HopInterval:      time.Millisecond,
		MaxPerHop:        1,
		ParallelRequests: 5,
		ProgressThrottle: time.Millisecond,
		DstIP:            dstIP,
	}, nil, func(result mtrProbeResult, _ int) {
		mu.Lock()
		callbackResults = append(callbackResults, result)
		mu.Unlock()
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Should have callbacks for TTL 1, 2, 3 (dest stops further TTLs)
	if len(callbackResults) < 3 {
		t.Errorf("expected at least 3 onProbe callbacks, got %d", len(callbackResults))
	}
}

func TestScheduler_BeginHopGreaterThanMaxHops(t *testing.T) {
	prober := &mockTTLProber{}
	agg := NewMTRAggregator()

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         10,
		MaxHops:          5,
		HopInterval:      time.Millisecond,
		MaxPerHop:        1,
		ParallelRequests: 1,
	}, nil, nil)

	if err == nil {
		t.Fatal("expected error for beginHop > maxHops")
	}
}

func TestScheduler_ConcurrencyLimit(t *testing.T) {
	var maxConcurrent int32
	var current int32

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			c := atomic.AddInt32(&current, 1)
			// Track max concurrent
			for {
				old := atomic.LoadInt32(&maxConcurrent)
				if c <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, c) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond) // hold slot
			atomic.AddInt32(&current, -1)
			return mtrProbeResult{TTL: ttl}, nil
		},
	}

	agg := NewMTRAggregator()

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          10,
		HopInterval:      time.Millisecond,
		MaxPerHop:        1,
		ParallelRequests: 3, // limit to 3 concurrent
		ProgressThrottle: time.Millisecond,
	}, nil, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mc := atomic.LoadInt32(&maxConcurrent)
	if mc > 3 {
		t.Errorf("expected max concurrent <= 3, got %d", mc)
	}
	if mc < 1 {
		t.Error("expected at least 1 concurrent probe")
	}
}

// TestMtrAddrToIP verifies the helper function.
func TestMtrAddrToIP(t *testing.T) {
	ip := net.ParseIP("1.2.3.4")

	if got := mtrAddrToIP(&net.IPAddr{IP: ip}); !got.Equal(ip) {
		t.Errorf("IPAddr: got %v, want %v", got, ip)
	}
	if got := mtrAddrToIP(&net.UDPAddr{IP: ip}); !got.Equal(ip) {
		t.Errorf("UDPAddr: got %v, want %v", got, ip)
	}
	if got := mtrAddrToIP(&net.TCPAddr{IP: ip}); !got.Equal(ip) {
		t.Errorf("TCPAddr: got %v, want %v", got, ip)
	}
	if got := mtrAddrToIP(nil); got != nil {
		t.Errorf("nil: got %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// P1: Error budget tests
// ---------------------------------------------------------------------------

func TestScheduler_ErrorBudgetExhausted(t *testing.T) {
	// Every call to ProbeTTL returns an error.
	// With MaxConsecErrors=3, MaxPerHop=2, each TTL should eventually
	// complete because every 3 consecutive errors count as one completed timeout.
	errAlways := errors.New("always fail")
	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			return mtrProbeResult{TTL: ttl}, errAlways
		},
	}

	agg := NewMTRAggregator()

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          2,
		HopInterval:      time.Millisecond,
		MaxPerHop:        2,
		MaxConsecErrors:  3,
		ParallelRequests: 2,
		ProgressThrottle: time.Millisecond,
	}, nil, nil)

	if err != nil {
		t.Fatalf("expected nil (completed), got %v", err)
	}

	// Each TTL should have 2 completed (timeout) events, each requiring 3 errors.
	// So total probes = 2 TTLs * 2 completions * 3 errors = 12.
	totalProbes := prober.getProbeCount()
	if totalProbes < 12 {
		t.Errorf("expected at least 12 probes (2 TTLs * 2 * 3 errors), got %d", totalProbes)
	}
}

func TestScheduler_ErrorBudgetEmitsOnProbe(t *testing.T) {
	// When error budget is exhausted, the synthetic timeout must also fire
	// the onProbe callback so that raw MTR sees the same record count as
	// the aggregator Snt.
	errAlways := errors.New("always fail")
	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			return mtrProbeResult{TTL: ttl}, errAlways
		},
	}

	agg := NewMTRAggregator()

	var mu sync.Mutex
	var rawRecords []mtrProbeResult

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          1,
		HopInterval:      time.Millisecond,
		MaxPerHop:        2,
		MaxConsecErrors:  3,
		ParallelRequests: 1,
		ProgressThrottle: time.Millisecond,
	}, nil, func(result mtrProbeResult, _ int) {
		mu.Lock()
		rawRecords = append(rawRecords, result)
		mu.Unlock()
	})

	if err != nil {
		t.Fatalf("expected nil (completed), got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// MaxPerHop=2, each requiring MaxConsecErrors=3 → 2 onProbe calls for TTL 1.
	if len(rawRecords) != 2 {
		t.Errorf("expected 2 onProbe callbacks (synthetic timeouts), got %d", len(rawRecords))
	}
	for i, r := range rawRecords {
		if r.TTL != 1 {
			t.Errorf("record[%d]: expected TTL=1, got %d", i, r.TTL)
		}
		if r.Success {
			t.Errorf("record[%d]: expected Success=false for timeout", i)
		}
	}
}

func TestScheduler_ErrorResetsOnSuccess(t *testing.T) {
	// Pattern: fail, fail, succeed, fail, fail, succeed — should never hit budget.
	var calls int32

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			n := atomic.AddInt32(&calls, 1)
			if n%3 != 0 { // every 3rd call succeeds
				return mtrProbeResult{TTL: ttl}, errors.New("fail")
			}
			return mtrProbeResult{
				TTL:     ttl,
				Success: true,
				Addr:    &net.IPAddr{IP: net.ParseIP("10.0.0.1")},
				RTT:     1 * time.Millisecond,
			}, nil
		},
	}

	agg := NewMTRAggregator()

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          1,
		HopInterval:      time.Millisecond,
		MaxPerHop:        2, // need 2 successful
		MaxConsecErrors:  3, // budget = 3 consecutive
		ParallelRequests: 1,
		ProgressThrottle: time.Millisecond,
	}, nil, nil)

	if err != nil {
		t.Fatalf("expected nil (completed via successes), got %v", err)
	}

	stats := agg.Snapshot()
	found := false
	for _, s := range stats {
		if s.TTL == 1 && s.Snt >= 2 {
			found = true
		}
	}
	if !found {
		t.Error("TTL 1 should have at least 2 successful probes in aggregator")
	}
}

// ---------------------------------------------------------------------------
// P2: Fallback geo/hostname propagation tests
// ---------------------------------------------------------------------------

func TestScheduler_FallbackGeoCarriedToAggregator(t *testing.T) {
	fakeGeo := &ipgeo.IPGeoData{
		Asnumber: "AS13335",
		Country:  "美国",
		Prov:     "加利福尼亚",
		City:     "旧金山",
		Owner:    "Cloudflare",
	}

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			return mtrProbeResult{
				TTL:      ttl,
				Success:  true,
				Addr:     &net.IPAddr{IP: net.ParseIP("1.1.1.1")},
				RTT:      5 * time.Millisecond,
				Hostname: "one.one.one.one",
				Geo:      fakeGeo,
			}, nil
		},
	}

	agg := NewMTRAggregator()

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          1,
		HopInterval:      time.Millisecond,
		MaxPerHop:        1,
		ParallelRequests: 1,
		ProgressThrottle: time.Millisecond,
		FillGeo:          true, // should NOT re-fetch since probe carries Geo
	}, nil, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := agg.Snapshot()
	if len(stats) == 0 {
		t.Fatal("expected at least 1 stat row")
	}

	s := stats[0]
	if s.Host != "one.one.one.one" {
		t.Errorf("expected Host='one.one.one.one', got %q", s.Host)
	}
	if s.Geo == nil {
		t.Fatal("expected Geo to be set, got nil")
	}
	if s.Geo.Asnumber != "AS13335" {
		t.Errorf("expected ASN='AS13335', got %q", s.Geo.Asnumber)
	}
}

func TestBuildMTRRawRecordFromProbe_PreResolvedGeo(t *testing.T) {
	fakeGeo := &ipgeo.IPGeoData{
		Asnumber:  "AS9808",
		Country:   "中国",
		CountryEn: "China",
		City:      "广州",
		CityEn:    "Guangzhou",
		Owner:     "ChinaMobile",
	}

	pr := mtrProbeResult{
		TTL:      3,
		Success:  true,
		Addr:     &net.IPAddr{IP: net.ParseIP("120.196.165.24")},
		RTT:      8 * time.Millisecond,
		Hostname: "bras-vlan365.gd.gd",
		Geo:      fakeGeo,
	}

	rec := buildMTRRawRecordFromProbe(5, pr, Config{Lang: "cn"})

	if rec.ASN != "AS9808" {
		t.Errorf("expected ASN='AS9808', got %q", rec.ASN)
	}
	if rec.Host != "bras-vlan365.gd.gd" {
		t.Errorf("expected Host='bras-vlan365.gd.gd', got %q", rec.Host)
	}
	if rec.City != "广州" {
		t.Errorf("expected City='广州', got %q", rec.City)
	}
	if rec.Country != "中国" {
		t.Errorf("expected Country='中国', got %q", rec.Country)
	}
	if rec.Owner != "ChinaMobile" {
		t.Errorf("expected Owner='ChinaMobile', got %q", rec.Owner)
	}
}

func TestBuildMTRRawRecordFromProbe_NoGeoNoSource_NoHostname(t *testing.T) {
	// When probe has no pre-resolved geo and config has no IPGeoSource/RDNS,
	// record should still have IP/RTT but no geo/host fields.
	pr := mtrProbeResult{
		TTL:     2,
		Success: true,
		Addr:    &net.IPAddr{IP: net.ParseIP("10.0.0.5")},
		RTT:     3 * time.Millisecond,
	}

	rec := buildMTRRawRecordFromProbe(1, pr, Config{})

	if rec.IP != "10.0.0.5" {
		t.Errorf("expected IP='10.0.0.5', got %q", rec.IP)
	}
	if rec.ASN != "" || rec.Host != "" || rec.Country != "" {
		t.Errorf("expected empty geo/host fields, got ASN=%q Host=%q Country=%q",
			rec.ASN, rec.Host, rec.Country)
	}
}

// ---------------------------------------------------------------------------
// End-to-end: raw record count matches aggregator Snt under error budget
// ---------------------------------------------------------------------------

func TestScheduler_RawRecordCountMatchesAggSnt_ErrorBudget(t *testing.T) {
	// Simulate a mix of successes and persistent errors across 2 TTLs.
	// TTL 1: always succeeds. TTL 2: always errors.
	// With MaxPerHop=3 and MaxConsecErrors=2, both TTLs should complete
	// and the raw callback count per TTL must equal the aggregator Snt.

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			if ttl == 1 {
				return mtrProbeResult{
					TTL:     1,
					Success: true,
					Addr:    &net.IPAddr{IP: net.ParseIP("10.0.0.1")},
					RTT:     5 * time.Millisecond,
				}, nil
			}
			return mtrProbeResult{TTL: 2}, errors.New("persistent failure")
		},
	}

	agg := NewMTRAggregator()

	var mu sync.Mutex
	rawCountByTTL := map[int]int{}

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          2,
		HopInterval:      time.Millisecond,
		MaxPerHop:        3,
		MaxConsecErrors:  2,
		ParallelRequests: 1,
		ProgressThrottle: time.Millisecond,
	}, nil, func(result mtrProbeResult, _ int) {
		mu.Lock()
		rawCountByTTL[result.TTL]++
		mu.Unlock()
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := agg.Snapshot()
	sntByTTL := map[int]int{}
	for _, s := range stats {
		sntByTTL[s.TTL] = s.Snt
	}

	mu.Lock()
	defer mu.Unlock()

	for ttl := 1; ttl <= 2; ttl++ {
		rawCount := rawCountByTTL[ttl]
		snt := sntByTTL[ttl]
		if rawCount != snt {
			t.Errorf("TTL %d: raw callback count (%d) != aggregator Snt (%d)",
				ttl, rawCount, snt)
		}
		if snt != 3 {
			t.Errorf("TTL %d: expected Snt=3, got %d", ttl, snt)
		}
	}
}

// ---------------------------------------------------------------------------
// RDNS-only: IPGeoSource=nil && RDNS=true enters fetchIPData path
// ---------------------------------------------------------------------------

func TestBuildMTRRawRecordFromProbe_RDNSOnlyPath(t *testing.T) {
	// With IPGeoSource=nil and RDNS=true, the function should enter the
	// fetchIPData path (not skip it). We use 127.0.0.1 which typically
	// resolves to "localhost" via PTR. Even if RDNS fails in CI, the test
	// verifies the code path doesn't panic and the record is well-formed.
	pr := mtrProbeResult{
		TTL:     1,
		Success: true,
		Addr:    &net.IPAddr{IP: net.ParseIP("127.0.0.1")},
		RTT:     1 * time.Millisecond,
	}

	rec := buildMTRRawRecordFromProbe(1, pr, Config{
		RDNS:        true,
		IPGeoSource: nil, // no geo source — only RDNS
		Lang:        "en",
	})

	// Basic sanity: record must have IP and RTT regardless.
	if rec.IP != "127.0.0.1" {
		t.Errorf("expected IP='127.0.0.1', got %q", rec.IP)
	}
	if rec.RTTMs <= 0 {
		t.Errorf("expected positive RTTMs, got %f", rec.RTTMs)
	}
	// Geo fields should be empty (no IPGeoSource).
	if rec.ASN != "" {
		t.Errorf("expected empty ASN with no geo source, got %q", rec.ASN)
	}
	// Host may or may not be set depending on system RDNS for 127.0.0.1.
	// The key assertion is that we reached here without panic/skip.
	t.Logf("RDNS-only path: Host=%q (may vary by system)", rec.Host)
}

// ---------------------------------------------------------------------------
// Destination folding tests
// ---------------------------------------------------------------------------

func TestScheduler_HigherTTLDestinationRepliesDiscarded(t *testing.T) {
	// Destination is at TTL 3. Higher TTLs also return the destination IP
	// but with a small delay, ensuring TTL 3's result is processed first
	// (setting knownFinalTTL=3). The delayed higher-TTL probes are now
	// DISCARDED — not folded into TTL 3.
	dstIP := net.ParseIP("10.0.0.99")

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			if ttl > 3 {
				// Higher TTLs return destination but after a delay,
				// so TTL 3's result is processed first.
				time.Sleep(30 * time.Millisecond)
				return mtrProbeResult{
					TTL:     ttl,
					Success: true,
					Addr:    &net.IPAddr{IP: dstIP},
					RTT:     time.Duration(ttl) * time.Millisecond,
				}, nil
			}
			if ttl == 3 {
				return mtrProbeResult{
					TTL:     ttl,
					Success: true,
					Addr:    &net.IPAddr{IP: dstIP},
					RTT:     time.Duration(ttl) * time.Millisecond,
				}, nil
			}
			return mtrProbeResult{
				TTL:     ttl,
				Success: true,
				Addr:    &net.IPAddr{IP: net.ParseIP("10.0.0.1")},
				RTT:     time.Duration(ttl) * time.Millisecond,
			}, nil
		},
	}

	agg := NewMTRAggregator()

	var mu sync.Mutex
	probeByTTL := map[int]int{}

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          6,
		HopInterval:      time.Millisecond,
		MaxPerHop:        3,
		ParallelRequests: 6, // enough to launch TTLs 1-6 simultaneously
		ProgressThrottle: time.Millisecond,
		DstIP:            dstIP,
	}, nil, func(result mtrProbeResult, _ int) {
		mu.Lock()
		probeByTTL[result.TTL]++
		mu.Unlock()
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := agg.Snapshot()
	sntByTTL := map[int]int{}
	for _, s := range stats {
		sntByTTL[s.TTL] = s.Snt
	}

	// TTL 3 (finalTTL) should have ONLY its own probes — Snt == MaxPerHop (no fold)
	if sntByTTL[3] != 3 {
		t.Errorf("TTL 3 (final): expected Snt == 3 (own probes only, no fold), got %d", sntByTTL[3])
	}

	// TTLs above final should NOT appear in aggregator (discarded)
	for ttl := 4; ttl <= 6; ttl++ {
		if sntByTTL[ttl] > 0 {
			t.Errorf("TTL %d: expected Snt=0 (discarded), got %d", ttl, sntByTTL[ttl])
		}
	}

	// onProbe callbacks must NOT fire for discarded higher-TTL results
	mu.Lock()
	defer mu.Unlock()
	for ttl := 4; ttl <= 6; ttl++ {
		if probeByTTL[ttl] > 0 {
			t.Errorf("onProbe: TTL %d should have 0 callbacks (discarded), got %d", ttl, probeByTTL[ttl])
		}
	}
	// Also verify that NO folded callbacks appeared at TTL 3 from higher-TTL probes:
	// TTL 3's callback count must equal its Snt.
	if probeByTTL[3] != sntByTTL[3] {
		t.Errorf("onProbe: TTL 3 callback count (%d) != Snt (%d); suggests folded callbacks leaked", probeByTTL[3], sntByTTL[3])
	}
}

func TestScheduler_DiscardedDestinationRepliesCannotExceedMaxPerHop(t *testing.T) {
	// With discard semantics, higher TTL destination replies are discarded
	// entirely, so they can never push finalTTL's Snt above MaxPerHop.
	dstIP := net.ParseIP("10.0.0.99")

	var probeCount int32
	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			atomic.AddInt32(&probeCount, 1)
			if ttl >= 2 {
				// All TTLs >= 2 hit destination
				return mtrProbeResult{
					TTL:     ttl,
					Success: true,
					Addr:    &net.IPAddr{IP: dstIP},
					RTT:     5 * time.Millisecond,
				}, nil
			}
			return mtrProbeResult{
				TTL:     ttl,
				Success: true,
				Addr:    &net.IPAddr{IP: net.ParseIP("10.0.0.1")},
				RTT:     1 * time.Millisecond,
			}, nil
		},
	}

	agg := NewMTRAggregator()

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          10,
		HopInterval:      time.Millisecond,
		MaxPerHop:        2, // strict cap
		ParallelRequests: 10,
		ProgressThrottle: time.Millisecond,
		DstIP:            dstIP,
	}, nil, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := agg.Snapshot()
	for _, s := range stats {
		if s.TTL == 2 {
			// TTL 2 is the finalTTL; higher TTL destination replies are
			// discarded entirely, so Snt must equal exactly MaxPerHop
			// (only own probes counted).
			if s.Snt > 2 {
				t.Errorf("TTL 2 (final): Snt=%d exceeds MaxPerHop=2 (discard violation)", s.Snt)
			}
		}
	}

	// Verify no higher TTLs have stats in the aggregator
	for _, s := range stats {
		if s.TTL > 2 && s.IP == dstIP.String() {
			t.Errorf("TTL %d: should not have dst-ip stats (discarded), got Snt=%d", s.TTL, s.Snt)
		}
	}
}

func TestScheduler_NonDestinationRepliesOnDisabledHigherTTLDiscarded(t *testing.T) {
	// If a higher TTL (after being disabled) returns a non-destination IP,
	// that reply should be silently discarded — not folded, not recorded.
	// Higher TTLs are delayed so TTL 3 (destination) is processed first.
	dstIP := net.ParseIP("10.0.0.99")

	var mu sync.Mutex
	var probeResults []mtrProbeResult

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			if ttl == 3 {
				// TTL 3 is destination — returns quickly
				return mtrProbeResult{
					TTL:     ttl,
					Success: true,
					Addr:    &net.IPAddr{IP: dstIP},
					RTT:     5 * time.Millisecond,
				}, nil
			}
			if ttl > 3 {
				// Higher TTLs return a non-destination intermediate IP
				// after a delay, so they arrive after TTL 3 sets disabled.
				time.Sleep(30 * time.Millisecond)
				return mtrProbeResult{
					TTL:     ttl,
					Success: true,
					Addr:    &net.IPAddr{IP: net.ParseIP("10.0.0.50")},
					RTT:     3 * time.Millisecond,
				}, nil
			}
			return mtrProbeResult{
				TTL:     ttl,
				Success: true,
				Addr:    &net.IPAddr{IP: net.ParseIP("10.0.0." + fmt.Sprintf("%d", ttl))},
				RTT:     time.Duration(ttl) * time.Millisecond,
			}, nil
		},
	}

	agg := NewMTRAggregator()

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          6,
		HopInterval:      time.Millisecond,
		MaxPerHop:        2,
		ParallelRequests: 6, // all TTLs may launch before destination detected
		ProgressThrottle: time.Millisecond,
		DstIP:            dstIP,
	}, nil, func(result mtrProbeResult, _ int) {
		mu.Lock()
		probeResults = append(probeResults, result)
		mu.Unlock()
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Non-destination replies from disabled TTLs (4, 5, 6) with IP 10.0.0.50
	// should have been discarded. Check that the aggregator has no entries
	// for TTLs > 3 with the intermediate IP.
	stats := agg.Snapshot()
	for _, s := range stats {
		if s.TTL > 3 && s.IP == "10.0.0.50" {
			t.Errorf("TTL %d: non-destination reply (10.0.0.50) should have been discarded, but appeared in aggregator", s.TTL)
		}
	}
}

func TestScheduler_FinalTTLLowered_MigratesStatsToNewFinal(t *testing.T) {
	// Scenario: higher TTL (12) returns destination first, establishing
	// knownFinalTTL=12. Then a lower TTL (7) returns destination, lowering
	// knownFinalTTL to 7. The stats already recorded at TTL 12 must be
	// migrated to TTL 7 — no ghost row at TTL 12 should remain.
	dstIP := net.ParseIP("10.0.0.99")

	var mu sync.Mutex
	callOrder := map[int]int{} // ttl → order of first return
	var callSeq int32

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			mu.Lock()
			if callOrder[ttl] == 0 {
				callOrder[ttl] = int(atomic.AddInt32(&callSeq, 1))
			}
			mu.Unlock()

			if ttl == 12 {
				// TTL 12 returns destination quickly
				return mtrProbeResult{
					TTL: ttl, Success: true,
					Addr: &net.IPAddr{IP: dstIP},
					RTT:  3 * time.Millisecond,
				}, nil
			}
			if ttl == 7 {
				// TTL 7 returns destination after a delay,
				// ensuring TTL 12 is processed first.
				time.Sleep(50 * time.Millisecond)
				return mtrProbeResult{
					TTL: ttl, Success: true,
					Addr: &net.IPAddr{IP: dstIP},
					RTT:  5 * time.Millisecond,
				}, nil
			}
			// Intermediate hops
			return mtrProbeResult{
				TTL: ttl, Success: true,
				Addr: &net.IPAddr{IP: net.ParseIP("10.0.0." + fmt.Sprintf("%d", ttl))},
				RTT:  time.Duration(ttl) * time.Millisecond,
			}, nil
		},
	}

	agg := NewMTRAggregator()

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          15,
		HopInterval:      time.Millisecond,
		MaxPerHop:        3,
		ParallelRequests: 15,
		ProgressThrottle: time.Millisecond,
		DstIP:            dstIP,
	}, nil, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := agg.Snapshot()
	sntByTTL := map[int]int{}
	ipByTTL := map[int]string{}
	for _, s := range stats {
		sntByTTL[s.TTL] = s.Snt
		ipByTTL[s.TTL] = s.IP
	}

	// TTL 12 should have NO stats — all migrated to TTL 7
	if sntByTTL[12] > 0 {
		t.Errorf("TTL 12 should have 0 stats after migration, got Snt=%d (ghost row!)", sntByTTL[12])
	}

	// TTL 7 (new final) should have stats (its own + migrated from 12)
	if sntByTTL[7] < 3 {
		t.Errorf("TTL 7 (final): expected Snt >= 3, got %d", sntByTTL[7])
	}

	// Only one row should have the destination IP
	dstIPRows := 0
	for _, s := range stats {
		if s.IP == "10.0.0.99" {
			dstIPRows++
			if s.TTL != 7 {
				t.Errorf("destination IP found at TTL %d, expected only at TTL 7", s.TTL)
			}
		}
	}
	if dstIPRows == 0 {
		t.Error("expected at least one row with destination IP")
	}
	if dstIPRows > 1 {
		t.Errorf("expected exactly 1 dst-ip row (at TTL 7), got %d (duplicate!)", dstIPRows)
	}
}

func TestScheduler_FinalTTLLowered_ChainMigration(t *testing.T) {
	// Chain scenario: TTL 12 → final. Then TTL 9 → final (migrates 12→9).
	// Then TTL 7 → final (migrates 9→7). All stats end up at TTL 7.
	dstIP := net.ParseIP("10.0.0.99")

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			if ttl == 12 {
				return mtrProbeResult{
					TTL: ttl, Success: true,
					Addr: &net.IPAddr{IP: dstIP},
					RTT:  3 * time.Millisecond,
				}, nil
			}
			if ttl == 9 {
				time.Sleep(30 * time.Millisecond)
				return mtrProbeResult{
					TTL: ttl, Success: true,
					Addr: &net.IPAddr{IP: dstIP},
					RTT:  4 * time.Millisecond,
				}, nil
			}
			if ttl == 7 {
				time.Sleep(60 * time.Millisecond)
				return mtrProbeResult{
					TTL: ttl, Success: true,
					Addr: &net.IPAddr{IP: dstIP},
					RTT:  5 * time.Millisecond,
				}, nil
			}
			return mtrProbeResult{
				TTL: ttl, Success: true,
				Addr: &net.IPAddr{IP: net.ParseIP("10.0.0." + fmt.Sprintf("%d", ttl))},
				RTT:  time.Duration(ttl) * time.Millisecond,
			}, nil
		},
	}

	agg := NewMTRAggregator()

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          15,
		HopInterval:      time.Millisecond,
		MaxPerHop:        2,
		ParallelRequests: 15,
		ProgressThrottle: time.Millisecond,
		DstIP:            dstIP,
	}, nil, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := agg.Snapshot()
	for _, s := range stats {
		if s.IP == "10.0.0.99" && s.TTL != 7 {
			t.Errorf("destination IP at TTL %d, expected only at TTL 7 after chain migration", s.TTL)
		}
	}

	// TTLs 9 and 12 should not have dst-ip stats
	for _, s := range stats {
		if (s.TTL == 9 || s.TTL == 12) && s.IP == "10.0.0.99" {
			t.Errorf("TTL %d: ghost row with dst-ip after chain migration", s.TTL)
		}
	}
}

// ---------------------------------------------------------------------------
// New regression tests: discard over-final destination replies
// ---------------------------------------------------------------------------

func TestScheduler_LateHigherTTLDestinationReply_Discarded_NoSntBump(t *testing.T) {
	// Scheduler dispatches multiple TTLs concurrently.
	// TTL 3 hits destination first → sets knownFinalTTL=3 and disables >3.
	// Later: originTTL=5 returns destination reply (late) → MUST be discarded.
	dstIP := net.ParseIP("10.0.0.99")

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			if ttl == 3 {
				// Destination — returns fast
				return mtrProbeResult{
					TTL: ttl, Success: true,
					Addr: &net.IPAddr{IP: dstIP},
					RTT:  3 * time.Millisecond,
				}, nil
			}
			if ttl == 5 {
				// Late destination reply — delayed so TTL 3 is processed first
				time.Sleep(40 * time.Millisecond)
				return mtrProbeResult{
					TTL: ttl, Success: true,
					Addr: &net.IPAddr{IP: dstIP},
					RTT:  8 * time.Millisecond,
				}, nil
			}
			return mtrProbeResult{
				TTL: ttl, Success: true,
				Addr: &net.IPAddr{IP: net.ParseIP(fmt.Sprintf("10.0.0.%d", ttl))},
				RTT:  time.Duration(ttl) * time.Millisecond,
			}, nil
		},
	}

	agg := NewMTRAggregator()

	var mu sync.Mutex
	var callbackCount int

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          6,
		HopInterval:      time.Millisecond,
		MaxPerHop:        2,
		ParallelRequests: 6,
		ProgressThrottle: time.Millisecond,
		DstIP:            dstIP,
	}, nil, func(result mtrProbeResult, _ int) {
		mu.Lock()
		callbackCount++
		mu.Unlock()
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := agg.Snapshot()
	sntByTTL := map[int]int{}
	for _, s := range stats {
		sntByTTL[s.TTL] = s.Snt
	}

	// Final hop TTL 3 should have exactly MaxPerHop (2) Snt — no bump from TTL 5
	if sntByTTL[3] != 2 {
		t.Errorf("TTL 3 (final): expected Snt=2 (MaxPerHop, own probes only), got %d", sntByTTL[3])
	}

	// TTL 5 should have 0 Snt — discarded
	if sntByTTL[5] > 0 {
		t.Errorf("TTL 5: expected Snt=0 (discarded late dst reply), got %d", sntByTTL[5])
	}

	// Callback count should equal sum of Snt across active TTLs only
	mu.Lock()
	totalSnt := 0
	for _, s := range stats {
		totalSnt += s.Snt
	}
	if callbackCount != totalSnt {
		t.Errorf("callback count (%d) != total Snt (%d); discarded results may have leaked", callbackCount, totalSnt)
	}
	mu.Unlock()
}

func TestScheduler_DiscardedOverFinal_DoesNotEmitOnProbe(t *testing.T) {
	// Provide onProbe hook that appends all records.
	// Trigger late over-final destination reply.
	// Assert no record appended for discarded result.
	dstIP := net.ParseIP("10.0.0.99")

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			if ttl == 2 {
				return mtrProbeResult{
					TTL: ttl, Success: true,
					Addr: &net.IPAddr{IP: dstIP},
					RTT:  5 * time.Millisecond,
				}, nil
			}
			if ttl > 2 {
				time.Sleep(30 * time.Millisecond)
				return mtrProbeResult{
					TTL: ttl, Success: true,
					Addr: &net.IPAddr{IP: dstIP},
					RTT:  10 * time.Millisecond,
				}, nil
			}
			return mtrProbeResult{
				TTL: ttl, Success: true,
				Addr: &net.IPAddr{IP: net.ParseIP("10.0.0.1")},
				RTT:  1 * time.Millisecond,
			}, nil
		},
	}

	agg := NewMTRAggregator()

	var mu sync.Mutex
	var records []mtrProbeResult

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          5,
		HopInterval:      time.Millisecond,
		MaxPerHop:        2,
		ParallelRequests: 5,
		ProgressThrottle: time.Millisecond,
		DstIP:            dstIP,
	}, nil, func(result mtrProbeResult, _ int) {
		mu.Lock()
		records = append(records, result)
		mu.Unlock()
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// No record should have TTL > 2 (all discarded)
	for i, r := range records {
		if r.TTL > 2 {
			t.Errorf("record[%d]: TTL %d should not have been emitted (discarded over-final)", i, r.TTL)
		}
	}

	// Record count must match aggregator Snt sum
	stats := agg.Snapshot()
	totalSnt := 0
	for _, s := range stats {
		totalSnt += s.Snt
	}
	if len(records) != totalSnt {
		t.Errorf("record count (%d) != total Snt (%d); 1:1 onProbe/Snt invariant violated", len(records), totalSnt)
	}
}

func TestScheduler_FinalTTLLowering_Chain_WithMaxPerHop_NoGhostRow_StableStats(t *testing.T) {
	// Construct deterministic RTT samples with chain lowering:
	// Provisional final at TTL 12 → lowered to 9 → lowered to 7.
	// MaxPerHop=3 (small). Verify no ghost rows, stats stable.
	dstIP := net.ParseIP("10.0.0.99")

	// RTT values for deterministic stat validation
	rttMap := map[int][]time.Duration{
		7:  {5 * time.Millisecond, 6 * time.Millisecond, 7 * time.Millisecond},
		9:  {10 * time.Millisecond, 11 * time.Millisecond},
		12: {20 * time.Millisecond},
	}
	var mu sync.Mutex
	callCountByTTL := map[int]int{}

	prober := &mockTTLProber{
		probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
			mu.Lock()
			callCountByTTL[ttl]++
			n := callCountByTTL[ttl]
			mu.Unlock()

			if ttl == 12 {
				// First to return destination
				rtt := 20 * time.Millisecond
				if n <= len(rttMap[12]) {
					rtt = rttMap[12][n-1]
				}
				return mtrProbeResult{
					TTL: ttl, Success: true,
					Addr: &net.IPAddr{IP: dstIP},
					RTT:  rtt,
				}, nil
			}
			if ttl == 9 {
				time.Sleep(30 * time.Millisecond)
				rtt := 11 * time.Millisecond
				if n <= len(rttMap[9]) {
					rtt = rttMap[9][n-1]
				}
				return mtrProbeResult{
					TTL: ttl, Success: true,
					Addr: &net.IPAddr{IP: dstIP},
					RTT:  rtt,
				}, nil
			}
			if ttl == 7 {
				time.Sleep(60 * time.Millisecond)
				rtt := 7 * time.Millisecond
				if n <= len(rttMap[7]) {
					rtt = rttMap[7][n-1]
				}
				return mtrProbeResult{
					TTL: ttl, Success: true,
					Addr: &net.IPAddr{IP: dstIP},
					RTT:  rtt,
				}, nil
			}
			return mtrProbeResult{
				TTL: ttl, Success: true,
				Addr: &net.IPAddr{IP: net.ParseIP(fmt.Sprintf("10.0.0.%d", ttl))},
				RTT:  time.Duration(ttl) * time.Millisecond,
			}, nil
		},
	}

	agg := NewMTRAggregator()

	err := runMTRScheduler(context.Background(), prober, agg, mtrSchedulerConfig{
		BeginHop:         1,
		MaxHops:          15,
		HopInterval:      time.Millisecond,
		MaxPerHop:        3,
		ParallelRequests: 15,
		ProgressThrottle: time.Millisecond,
		DstIP:            dstIP,
	}, nil, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := agg.Snapshot()
	sntByTTL := map[int]int{}
	ipByTTL := map[int]string{}
	var finalHopStat *MTRHopStat
	for i, s := range stats {
		sntByTTL[s.TTL] = s.Snt
		ipByTTL[s.TTL] = s.IP
		if s.TTL == 7 && s.IP == dstIP.String() {
			finalHopStat = &stats[i]
		}
	}

	// No ghost rows at TTL 12 or 9 with destination IP
	for _, ttl := range []int{12, 9} {
		if ipByTTL[ttl] == dstIP.String() {
			t.Errorf("TTL %d: ghost row with dst-ip after chain migration", ttl)
		}
	}

	// Final hop is TTL 7
	if finalHopStat == nil {
		t.Fatal("expected final hop at TTL 7 with destination IP")
	}

	// Snt <= MaxPerHop
	if finalHopStat.Snt > 3 {
		t.Errorf("TTL 7 (final): Snt=%d exceeds MaxPerHop=3", finalHopStat.Snt)
	}

	// Snt must be > 0 (at least the migrated + own)
	if finalHopStat.Snt == 0 {
		t.Error("TTL 7 (final): Snt=0, expected > 0 after migration")
	}

	// Avg should be reasonable (> 0 and not NaN)
	if finalHopStat.Avg <= 0 {
		t.Errorf("TTL 7 (final): Avg=%f, expected > 0 (stable stats)", finalHopStat.Avg)
	}

	// StDev should be non-negative
	if finalHopStat.StDev < 0 {
		t.Errorf("TTL 7 (final): StDev=%f, expected >= 0 (stable stats)", finalHopStat.StDev)
	}

	// Destination IP should appear exactly once across all stats rows
	dstIPCount := 0
	for _, s := range stats {
		if s.IP == dstIP.String() {
			dstIPCount++
			if s.TTL != 7 {
				t.Errorf("destination IP found at TTL %d, expected only at TTL 7", s.TTL)
			}
		}
	}
	if dstIPCount != 1 {
		t.Errorf("expected exactly 1 row with dst-ip (at TTL 7), got %d", dstIPCount)
	}
}
