package pow

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tsosunchia/powclient"
)

func TestGetToken(t *testing.T) {
	// 网络可达性前置检查：尝试 TCP 连接目标服务器
	conn, err := net.DialTimeout("tcp", "origin-fallback.nxtrace.org:443", 3*time.Second)
	if err != nil {
		t.Skipf("skipping: network unreachable (origin-fallback.nxtrace.org:443): %v", err)
	}
	conn.Close()

	// 计时开始
	start := time.Now()
	token, err := GetToken("origin-fallback.nxtrace.org", "origin-fallback.nxtrace.org", "443")
	// 计时结束
	end := time.Now()
	fmt.Println("耗时：", end.Sub(start))
	fmt.Println("token:", token)
	assert.NoError(t, err, "GetToken() returned an error")
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
