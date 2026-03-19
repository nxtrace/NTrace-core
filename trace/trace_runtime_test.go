package trace

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
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

func TestWaitForPendingGeoDataReturnsOnCanceledContext(t *testing.T) {
	res := &Result{
		Hops: [][]Hop{{
			{
				Address: &net.IPAddr{IP: net.ParseIP("1.1.1.1")},
				Geo:     pendingGeo(),
			},
		}},
	}
	res.geoWG.Add(1)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer res.geoWG.Done()
		<-done
	}()

	start := time.Now()
	cancel()
	waitForPendingGeoData(ctx, res)
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("waitForPendingGeoData returned too slowly after cancel: %v", elapsed)
	}
	if !res.geoCanceled.Load() {
		t.Fatal("geoCanceled = false, want true")
	}
	geo := res.Hops[0][0].Geo
	if geo == nil || geo.Source != timeoutGeoSource {
		t.Fatalf("hop geo = %+v, want timeout geo", geo)
	}
	close(done)
}

func TestWaitForPendingGeoDataReturnsImmediatelyForCompletedWorkers(t *testing.T) {
	res := &Result{
		Hops: [][]Hop{{
			{
				Address: &net.IPAddr{IP: net.ParseIP("1.1.1.1")},
				Geo:     &ipgeo.IPGeoData{CountryEn: "US"},
			},
		}},
	}

	start := time.Now()
	waitForPendingGeoData(context.Background(), res)
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("waitForPendingGeoData returned too slowly for completed result: %v", elapsed)
	}
}
