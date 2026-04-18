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
	sampled := make(chan struct{}, 1)
	p := StartLoaded(context.Background(), func(ctx context.Context) (float64, error) {
		time.Sleep(5 * time.Millisecond)
		select {
		case sampled <- struct{}{}:
		default:
		}
		return 1, nil
	})
	select {
	case <-sampled:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for loaded latency sample")
	}
	stats := p.Stop()
	if stats.N == 0 {
		t.Fatal("stats.N = 0, want > 0")
	}
}

func TestComputeJitterUsesSampleOrder(t *testing.T) {
	stats := Compute([]float64{10, 30, 20})
	if stats.Jitter != 15 {
		t.Fatalf("stats.Jitter = %v, want 15", stats.Jitter)
	}
}
