package tracemap

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/util"
)

type blockingRoundTripper struct{}

func (blockingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}

func TestGetMapUrlWithContextReturnsCanceled(t *testing.T) {
	oldHostPort := util.EnvHostPort
	oldFastIPFn := getFastIPWithContextFn
	oldClientFn := traceMapHTTPClientFn
	defer func() {
		util.EnvHostPort = oldHostPort
		getFastIPWithContextFn = oldFastIPFn
		traceMapHTTPClientFn = oldClientFn
	}()

	util.EnvHostPort = "example.com:443"
	getFastIPWithContextFn = func(ctx context.Context, domain string, port string, enableOutput bool) (string, error) {
		return "127.0.0.1", nil
	}
	traceMapHTTPClientFn = func(host string) *http.Client {
		return &http.Client{Transport: blockingRoundTripper{}}
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := GetMapUrlWithContext(ctx, `{"hops":[]}`)
		done <- err
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("GetMapUrlWithContext error = %v, want context.Canceled", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("GetMapUrlWithContext did not return promptly after cancel")
	}
}
