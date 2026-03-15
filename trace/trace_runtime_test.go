package trace

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/sync/semaphore"
)

func TestWaitForTraceDelayCanceledContextReturnsImmediately(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	if waitForTraceDelay(ctx, time.Second) {
		t.Fatal("waitForTraceDelay should return false for canceled context")
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("waitForTraceDelay returned too slowly after cancel: %v", elapsed)
	}
}

func TestWaitForTraceDelayZeroDelaySucceeds(t *testing.T) {
	if !waitForTraceDelay(context.Background(), 0) {
		t.Fatal("waitForTraceDelay should succeed for zero delay")
	}
}

func TestAcquireTraceSemaphoreChecksCanceledContextFirst(t *testing.T) {
	sem := semaphore.NewWeighted(1)
	if err := sem.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("initial acquire failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := acquireTraceSemaphore(ctx, sem)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("acquireTraceSemaphore error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("acquireTraceSemaphore returned too slowly after cancel: %v", elapsed)
	}

	sem.Release(1)
	if err := acquireTraceSemaphore(context.Background(), sem); err != nil {
		t.Fatalf("acquireTraceSemaphore should still acquire after release: %v", err)
	}
	sem.Release(1)
}
