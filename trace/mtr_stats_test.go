package trace

import (
	"math"
	"net"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

func mkHop(ttl int, ip string, rtt time.Duration) Hop {
	return Hop{
		Success: true,
		Address: &net.IPAddr{IP: net.ParseIP(ip)},
		TTL:     ttl,
		RTT:     rtt,
	}
}

func mkTimeoutHop(ttl int) Hop {
	return Hop{
		Success: false,
		Address: nil,
		TTL:     ttl,
		RTT:     0,
	}
}

func mkResult(hopsByTTL ...[]Hop) *Result {
	res := &Result{
		Hops: make([][]Hop, len(hopsByTTL)),
	}
	for i, hops := range hopsByTTL {
		res.Hops[i] = hops
	}
	return res
}

func roundN(v float64, n int) float64 {
	pow := math.Pow(10, float64(n))
	return math.Round(v*pow) / pow
}

func TestSinglePath(t *testing.T) {
	agg := NewMTRAggregator()

	res1 := mkResult(
		[]Hop{mkHop(1, "1.1.1.1", 10*time.Millisecond)},
		[]Hop{mkHop(2, "2.2.2.2", 20*time.Millisecond)},
	)
	agg.Update(res1, 1)

	res2 := mkResult(
		[]Hop{mkHop(1, "1.1.1.1", 12*time.Millisecond)},
		[]Hop{mkHop(2, "2.2.2.2", 18*time.Millisecond)},
	)
	agg.Update(res2, 1)

	res3 := mkResult(
		[]Hop{mkHop(1, "1.1.1.1", 14*time.Millisecond)},
		[]Hop{mkHop(2, "2.2.2.2", 22*time.Millisecond)},
	)
	stats := agg.Update(res3, 1)

	if len(stats) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(stats))
	}

	s := stats[0]
	if s.TTL != 1 {
		t.Errorf("TTL: want 1, got %d", s.TTL)
	}
	if s.Snt != 3 {
		t.Errorf("Snt: want 3, got %d", s.Snt)
	}
	if s.Received != 3 {
		t.Errorf("Received: want 3, got %d", s.Received)
	}
	if s.Loss != 0 {
		t.Errorf("Loss: want 0, got %f", s.Loss)
	}
	if s.Last != 14 {
		t.Errorf("Last: want 14, got %f", s.Last)
	}
	if s.Best != 10 {
		t.Errorf("Best: want 10, got %f", s.Best)
	}
	if s.Wrst != 14 {
		t.Errorf("Wrst: want 14, got %f", s.Wrst)
	}
	if roundN(s.Avg, 4) != 12 {
		t.Errorf("Avg: want 12, got %f", s.Avg)
	}
	if roundN(s.StDev, 4) != 2.0 {
		t.Errorf("StDev: want 2.0, got %f", s.StDev)
	}

	s2 := stats[1]
	if s2.TTL != 2 {
		t.Errorf("TTL: want 2, got %d", s2.TTL)
	}
	if s2.Snt != 3 {
		t.Errorf("Snt: want 3, got %d", s2.Snt)
	}
	if roundN(s2.Avg, 4) != 20 {
		t.Errorf("Avg: want 20, got %f", s2.Avg)
	}
	if roundN(s2.StDev, 4) != 2.0 {
		t.Errorf("StDev: want 2.0, got %f", s2.StDev)
	}
}

func TestMultiPath(t *testing.T) {
	agg := NewMTRAggregator()
	res1 := mkResult(
		[]Hop{
			mkHop(1, "10.0.0.1", 5*time.Millisecond),
			mkHop(1, "10.0.0.2", 8*time.Millisecond),
		},
	)
	stats := agg.Update(res1, 2)
	if len(stats) != 2 {
		t.Fatalf("expected 2 rows for multipath, got %d", len(stats))
	}
	if stats[0].TTL != 1 || stats[1].TTL != 1 {
		t.Errorf("both rows should be TTL=1")
	}
	ips := map[string]bool{stats[0].IP: true, stats[1].IP: true}
	if !ips["10.0.0.1"] || !ips["10.0.0.2"] {
		t.Errorf("expected both IPs")
	}

	res2 := mkResult([]Hop{mkHop(1, "10.0.0.1", 6*time.Millisecond)})
	stats = agg.Update(res2, 1)
	for _, s := range stats {
		if s.IP == "10.0.0.1" && s.Snt != 2 {
			t.Errorf("10.0.0.1 sent: want 2, got %d", s.Snt)
		}
		if s.IP == "10.0.0.2" && s.Snt != 1 {
			t.Errorf("10.0.0.2 sent: want 1, got %d", s.Snt)
		}
	}
}

func TestErrorMix(t *testing.T) {
	agg := NewMTRAggregator()
	res := mkResult(
		[]Hop{
			mkHop(1, "1.1.1.1", 10*time.Millisecond),
			mkHop(1, "1.1.1.1", 20*time.Millisecond),
			mkTimeoutHop(1),
		},
	)
	stats := agg.Update(res, 3)
	var found bool
	for _, s := range stats {
		if s.IP == "1.1.1.1" {
			found = true
			if s.Snt != 2 {
				t.Errorf("Snt: want 2, got %d", s.Snt)
			}
			if s.Received != 2 {
				t.Errorf("Received: want 2, got %d", s.Received)
			}
			if s.Loss != 0 {
				t.Errorf("Loss of 1.1.1.1: want 0, got %f", s.Loss)
			}
		}
	}
	if !found {
		t.Error("did not find stats for 1.1.1.1")
	}
	for _, s := range stats {
		if s.IP == "" && s.Host == "" {
			if s.Snt != 1 {
				t.Errorf("timeout Snt: want 1, got %d", s.Snt)
			}
			if s.Loss != 100 {
				t.Errorf("timeout Loss: want 100, got %f", s.Loss)
			}
		}
	}
}

func TestGeoPropagation(t *testing.T) {
	agg := NewMTRAggregator()
	geoData := &ipgeo.IPGeoData{Country: "US", City: "San Francisco"}
	hop := mkHop(1, "1.1.1.1", 10*time.Millisecond)
	hop.Geo = geoData
	res := mkResult([]Hop{hop})
	stats := agg.Update(res, 1)
	if len(stats) != 1 {
		t.Fatalf("expected 1 row, got %d", len(stats))
	}
	if stats[0].Geo == nil {
		t.Fatal("expected geo data, got nil")
	}
	if stats[0].Geo.Country != "US" {
		t.Errorf("Country: want US, got %s", stats[0].Geo.Country)
	}
}

func TestStDevSingleSample(t *testing.T) {
	agg := NewMTRAggregator()
	res := mkResult([]Hop{mkHop(1, "1.1.1.1", 10*time.Millisecond)})
	stats := agg.Update(res, 1)
	if len(stats) != 1 {
		t.Fatalf("expected 1 row, got %d", len(stats))
	}
	if stats[0].StDev != 0 {
		t.Errorf("StDev with 1 sample: want 0, got %f", stats[0].StDev)
	}
}

func TestAllTimeout(t *testing.T) {
	agg := NewMTRAggregator()
	res := mkResult([]Hop{mkTimeoutHop(1), mkTimeoutHop(1), mkTimeoutHop(1)})
	stats := agg.Update(res, 3)
	if len(stats) != 1 {
		t.Fatalf("expected 1 row, got %d", len(stats))
	}
	s := stats[0]
	if s.Snt != 3 {
		t.Errorf("Snt: want 3, got %d", s.Snt)
	}
	if s.Received != 0 {
		t.Errorf("Received: want 0, got %d", s.Received)
	}
	if s.Loss != 100 {
		t.Errorf("Loss: want 100, got %f", s.Loss)
	}
	if s.Avg != 0 || s.Best != 0 || s.Wrst != 0 || s.StDev != 0 {
		t.Errorf("all RTT should be 0 for all-timeout")
	}
}

func TestHostnamePropagation(t *testing.T) {
	agg := NewMTRAggregator()
	hop := mkHop(1, "1.1.1.1", 10*time.Millisecond)
	hop.Hostname = "one.one.one.one"
	res := mkResult([]Hop{hop})
	stats := agg.Update(res, 1)
	if len(stats) != 1 {
		t.Fatalf("expected 1 row, got %d", len(stats))
	}
	if stats[0].Host != "one.one.one.one" {
		t.Errorf("Host: want one.one.one.one, got %s", stats[0].Host)
	}
}

func TestMTRAggregator_Reset(t *testing.T) {
	agg := NewMTRAggregator()
	res := mkResult(
		[]Hop{mkHop(1, "1.1.1.1", 10*time.Millisecond)},
		[]Hop{mkHop(2, "2.2.2.2", 20*time.Millisecond)},
	)
	agg.Update(res, 1)
	agg.Update(res, 1)

	// Reset 后 Snapshot 应为空
	agg.Reset()
	snap := agg.Snapshot()
	if len(snap) != 0 {
		t.Fatalf("expected 0 rows after Reset, got %d", len(snap))
	}

	// Reset 后继续 Update 应正常工作，Snt 从 1 重新开始
	stats := agg.Update(res, 1)
	if len(stats) != 2 {
		t.Fatalf("expected 2 rows after re-update, got %d", len(stats))
	}
	if stats[0].Snt != 1 {
		t.Errorf("Snt after reset: want 1, got %d", stats[0].Snt)
	}
}

func TestMTRAggregator_CloneIsolation(t *testing.T) {
	agg := NewMTRAggregator()
	res1 := mkResult(
		[]Hop{mkHop(1, "1.1.1.1", 10*time.Millisecond)},
	)
	agg.Update(res1, 1)

	// Clone
	clone := agg.Clone()

	// 修改原始聚合器
	res2 := mkResult(
		[]Hop{mkHop(1, "1.1.1.1", 20*time.Millisecond)},
	)
	agg.Update(res2, 1)

	// Clone 快照应只有 1 次发送
	cloneSnap := clone.Snapshot()
	if len(cloneSnap) != 1 {
		t.Fatalf("clone: expected 1 row, got %d", len(cloneSnap))
	}
	if cloneSnap[0].Snt != 1 {
		t.Errorf("clone Snt: want 1, got %d", cloneSnap[0].Snt)
	}

	// 原始聚合器应有 2 次发送
	origSnap := agg.Snapshot()
	if origSnap[0].Snt != 2 {
		t.Errorf("original Snt: want 2, got %d", origSnap[0].Snt)
	}

	// 修改 Clone 不影响原始
	res3 := mkResult(
		[]Hop{mkHop(1, "1.1.1.1", 30*time.Millisecond)},
	)
	clone.Update(res3, 1)
	cloneSnap = clone.Snapshot()
	if cloneSnap[0].Snt != 2 {
		t.Errorf("clone Snt after update: want 2, got %d", cloneSnap[0].Snt)
	}
	origSnap = agg.Snapshot()
	if origSnap[0].Snt != 2 {
		t.Errorf("original Snt should still be 2, got %d", origSnap[0].Snt)
	}
}
