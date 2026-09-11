package trace

import (
	"math"
	"net"
	"reflect"
	"testing"
	"time"
)

func variabilityProbe(agg *MTRAggregator, ttl int, ip string, ms float64) {
	hop := Hop{TTL: ttl}
	if ip != "" {
		hop.Success = true
		hop.Address = &net.IPAddr{IP: net.ParseIP(ip)}
		hop.RTT = time.Duration(ms * float64(time.Millisecond))
	}
	agg.Update(mtrGoldenResult(ttl, hop), 1)
}

func variabilityValues(s MTRHopStat) []float64 {
	return []float64{s.GMean, s.Jitter, s.JitterAvg, s.JitterMax, s.JitterInterarrival}
}

func requireVariability(t *testing.T, got MTRHopStat, want []float64) {
	t.Helper()
	for i, value := range variabilityValues(got) {
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value-want[i]) > 1e-9*math.Max(1, math.Abs(want[i])) {
			t.Fatalf("metric %d: got %v want %v", i, value, want[i])
		}
	}
}

func TestMTRVariabilitySuccessfulSequence(t *testing.T) {
	for _, tt := range []struct {
		name          string
		samples, want []float64
	}{
		{"first", []float64{10}, []float64{10, 0, 0, 0, 0}},
		{"constant", []float64{7, 7, 7}, []float64{7, 0, 0, 0, 0}},
		{"increasing", []float64{10, 20, 40}, []float64{20, 20, 10, 20, 29.375}},
		{"zero", []float64{0, 10, 5}, []float64{0, 5, 5, 10, 14.375}},
		{"submicrosecond", []float64{0.000001, 0.000004}, []float64{0.000002, 0.000003, 0.0000015, 0.000003, 0.000003}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			agg := NewMTRAggregator()
			variabilityProbe(agg, 1, "", 0)
			for _, ms := range tt.samples {
				variabilityProbe(agg, 1, "192.0.2.1", ms)
			}
			variabilityProbe(agg, 1, "", 0)
			stat := agg.Snapshot()[0]
			requireVariability(t, stat, tt.want)
			if stat.Dropped != 2 || stat.Snt != len(tt.samples)+2 || stat.Received != len(tt.samples) {
				t.Fatal(stat)
			}
		})
	}
	agg := NewMTRAggregator()
	variabilityProbe(agg, 1, "", 0)
	requireVariability(t, agg.Snapshot()[0], []float64{0, 0, 0, 0, 0})
	if agg.Snapshot()[0].Dropped != 1 {
		t.Fatal(agg.Snapshot())
	}
}

func TestMTRVariabilityResponderIsolation(t *testing.T) {
	agg := NewMTRAggregator()
	variabilityProbe(agg, 1, "192.0.2.1", 10)
	variabilityProbe(agg, 1, "192.0.2.2", 100)
	variabilityProbe(agg, 1, "", 0)
	variabilityProbe(agg, 1, "192.0.2.1", 14)
	stats := agg.Snapshot()
	requireVariability(t, stats[0], []float64{math.Sqrt(140), 4, 2, 4, 4})
	requireVariability(t, stats[1], []float64{100, 0, 0, 0, 0})
	if len(stats) != 3 || stats[2].Dropped != 1 {
		t.Fatal(stats)
	}
	before := agg.Snapshot()
	clone := agg.Clone()
	variabilityProbe(clone, 1, "192.0.2.1", 20)
	if !reflect.DeepEqual(agg.Snapshot(), before) {
		t.Fatal("clone contaminated original")
	}
	agg.ClearHop(1)
	variabilityProbe(agg, 1, "192.0.2.1", 30)
	requireVariability(t, agg.Snapshot()[0], []float64{30, 0, 0, 0, 0})
	agg.Reset()
	variabilityProbe(agg, 1, "192.0.2.1", 40)
	requireVariability(t, agg.Snapshot()[0], []float64{40, 0, 0, 0, 0})
}

func TestMTRVariabilityMergeMatchesConcatenation(t *testing.T) {
	for _, samples := range [][]float64{{10, 20, 40, 15, 30}, {0, 10, 4, 0}, {10, 10, 10}} {
		full := NewMTRAggregator()
		for _, ms := range samples {
			variabilityProbe(full, 1, "192.0.2.1", ms)
		}
		for split := 0; split <= len(samples); split++ {
			joined := NewMTRAggregator()
			// Include empty-success summaries on either side as well as real sequences.
			variabilityProbe(joined, 1, "", 0)
			for i, ms := range samples {
				ttl := 1
				if i >= split {
					ttl = 2
				}
				variabilityProbe(joined, ttl, "192.0.2.1", ms)
			}
			joined.MigrateStats(2, 1, 0)
			stats := joined.Snapshot()
			for _, stat := range stats {
				if stat.Received > 0 {
					requireVariability(t, stat, variabilityValues(full.Snapshot()[0]))
				}
			}
		}
	}
}

func TestMTRVariabilityCapPreservesDerivedStatistics(t *testing.T) {
	agg := NewMTRAggregator()
	for _, ms := range []float64{10, 20, 40, 80} {
		variabilityProbe(agg, 2, "192.0.2.1", ms)
	}
	before := agg.Snapshot()[0]
	agg.MigrateStats(2, 1, 1)
	capped := agg.Snapshot()[0]
	requireVariability(t, capped, variabilityValues(before))
	if capped.Snt != 1 || capped.Received != 1 || capped.Dropped != 0 {
		t.Fatal(capped)
	}
	variabilityProbe(agg, 1, "192.0.2.1", 40)
	requireVariability(t, agg.Snapshot()[0], []float64{math.Sqrt(before.GMean * 40), 40, (before.JitterAvg + 40) / 2, 40, before.JitterInterarrival*15/16 + 40})
	variabilityProbe(agg, 2, "192.0.2.1", 20)
	agg.MigrateStats(2, 1, 0)
	for _, v := range variabilityValues(agg.Snapshot()[0]) {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatal(v)
		}
	}
}
