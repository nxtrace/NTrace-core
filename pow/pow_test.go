package pow

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tsosunchia/powclient"
)

func TestGetToken(t *testing.T) {
	oldRetTokenFn := retTokenFn
	defer func() { retTokenFn = oldRetTokenFn }()

	const (
		fastIP    = "origin-fallback.nxtrace.org"
		host      = "origin-fallback.nxtrace.org"
		port      = "443"
		wantToken = "test-token"
	)

	retTokenFn = func(params *powclient.GetTokenParams) (string, error) {
		if params == nil {
			t.Fatal("retToken params = nil")
		}
		wantURL := (&url.URL{Scheme: "https", Host: fastIP + ":" + port, Path: baseURL}).String()
		if params.BaseUrl != wantURL {
			t.Fatalf("BaseUrl = %q, want %q", params.BaseUrl, wantURL)
		}
		if params.SNI != host {
			t.Fatalf("SNI = %q, want %q", params.SNI, host)
		}
		if params.Host != host {
			t.Fatalf("Host = %q, want %q", params.Host, host)
		}
		if params.UserAgent == "" {
			t.Fatal("UserAgent should not be empty")
		}
		if params.TimeoutSec <= 0 {
			t.Fatalf("TimeoutSec = %v, want > 0", params.TimeoutSec)
		}
		return wantToken, nil
	}

	// 计时开始
	start := time.Now()
	token, err := GetToken(fastIP, host, port)
	// 计时结束
	end := time.Now()
	fmt.Println("耗时：", end.Sub(start))
	fmt.Println("token:", token)
	assert.NoError(t, err, "GetToken() returned an error")
	assert.Equal(t, wantToken, token)
}

func TestGetTokenIntegration(t *testing.T) {
	if os.Getenv("NTRACE_RUN_POW_INTEGRATION") != "1" {
		t.Skip("set NTRACE_RUN_POW_INTEGRATION=1 to run live PoW integration test")
	}

	conn, err := net.DialTimeout("tcp", "origin-fallback.nxtrace.org:443", 3*time.Second)
	if err != nil {
		t.Fatalf("network unreachable (origin-fallback.nxtrace.org:443): %v", err)
	}
	conn.Close()

	token, err := GetToken("origin-fallback.nxtrace.org", "origin-fallback.nxtrace.org", "443")
	assert.NoError(t, err, "GetToken() returned an error")
	assert.NotEmpty(t, token, "GetToken() returned empty token")
}

func TestGetTokenWithContextReturnsCanceled(t *testing.T) {
	oldRetTokenFn := retTokenFn
	defer func() { retTokenFn = oldRetTokenFn }()

	started := make(chan struct{})
	retTokenFn = func(*powclient.GetTokenParams) (string, error) {
		close(started)
		time.Sleep(200 * time.Millisecond)
		return "", errors.New("boom")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := GetTokenWithContext(ctx, "example.com", "example.com", "443")
		done <- err
	}()

	<-started
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("GetTokenWithContext error = %v, want context.Canceled", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("GetTokenWithContext did not return promptly after cancel")
	}
}

func TestGetTokenWithContextClampsRequestTimeoutToContextDeadline(t *testing.T) {
	oldRetTokenFn := retTokenFn
	defer func() { retTokenFn = oldRetTokenFn }()

	gotTimeout := make(chan time.Duration, 1)
	retTokenFn = func(params *powclient.GetTokenParams) (string, error) {
		select {
		case gotTimeout <- params.TimeoutSec:
		default:
		}
		return "", errors.New("boom")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	_, _ = GetTokenWithContext(ctx, "example.com", "example.com", "443")

	select {
	case timeout := <-gotTimeout:
		if timeout <= 0 {
			t.Fatalf("retToken timeout = %v, want > 0", timeout)
		}
		if timeout > 150*time.Millisecond {
			t.Fatalf("retToken timeout = %v, want <= 150ms", timeout)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("retTokenFn was not called")
	}
}
