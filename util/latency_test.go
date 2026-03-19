package util

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestGetFastIPWithContextReturnsCanceled(t *testing.T) {
	oldLookup := fastIPLookupHostFn
	oldCheck := fastIPCheckLatency
	oldCache := FastIpCache
	oldMeta := FastIPMetaCache
	defer func() {
		fastIPLookupHostFn = oldLookup
		fastIPCheckLatency = oldCheck
		FastIpCache = oldCache
		FastIPMetaCache = oldMeta
	}()

	FastIpCache = ""
	FastIPMetaCache = FastIPMeta{}
	fastIPLookupHostFn = func(ctx context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("1.1.1.1")}, nil
	}
	started := make(chan struct{})
	fastIPCheckLatency = func(ctx context.Context, domain, ip, port string, results chan<- ResponseInfo) {
		close(started)
		<-ctx.Done()
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := GetFastIPWithContext(ctx, "example.com", "443", false)
		done <- err
	}()

	<-started
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("GetFastIPWithContext error = %v, want context.Canceled", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("GetFastIPWithContext did not return promptly after cancel")
	}
}
