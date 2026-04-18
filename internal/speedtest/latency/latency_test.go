package latency

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCompute(t *testing.T) {
	stats := Compute([]float64{10, 20, 30})
	if stats.N != 3 || stats.Min != 10 || stats.Max != 30 || stats.Median != 20 || stats.Avg != 20 {
		t.Fatalf("Compute() = %+v, want consistent stats", stats)
	}
}

func TestMeasureIdleSkipsErrors(t *testing.T) {
	count := 0
	stats := MeasureIdle(context.Background(), 4, func(ctx context.Context) (float64, error) {
		count++
		if count%2 == 0 {
			return -1, errors.New("boom")
		}
		return float64(count), nil
	})
	if stats.N != 2 {
		t.Fatalf("stats.N = %d, want 2", stats.N)
	}
}

func TestStartLoadedCollectsSamplesUntilStopped(t *testing.T) {
	p := StartLoaded(context.Background(), func(ctx context.Context) (float64, error) {
		time.Sleep(5 * time.Millisecond)
		return 1, nil
	})
	time.Sleep(20 * time.Millisecond)
	stats := p.Stop()
	if stats.N == 0 {
		t.Fatal("stats.N = 0, want > 0")
	}
}
