package pow

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/tsosunchia/powclient"

	"github.com/nxtrace/NTrace-core/util"
)

const (
	baseURL = "/v3/challenge"
)

var retTokenFn = powclient.RetToken

func resolveTokenRequestTimeout(ctx context.Context, fallback time.Duration) time.Duration {
	if fallback <= 0 {
		fallback = 5 * time.Second
	}
	if ctx == nil {
		return fallback
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return fallback
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return time.Millisecond
	}
	if remaining < fallback {
		return remaining
	}
	return fallback
}

func GetToken(fastIp string, host string, port string) (string, error) {
	return GetTokenWithContext(context.Background(), fastIp, host, port)
}

func GetTokenWithContext(ctx context.Context, fastIp string, host string, port string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	getTokenParams := powclient.NewGetTokenParams()
	u := url.URL{Scheme: "https", Host: fastIp + ":" + port, Path: baseURL}
	getTokenParams.BaseUrl = u.String()
	getTokenParams.SNI = host
	getTokenParams.Host = host
	getTokenParams.UserAgent = util.UserAgent
	getTokenParams.TimeoutSec = resolveTokenRequestTimeout(opCtx, getTokenParams.TimeoutSec)
	proxyUrl := util.GetProxy()
	if proxyUrl != nil {
		getTokenParams.Proxy = proxyUrl
	}
	var (
		token string
		err   error
		done  = make(chan error, 1)
	)
	go func() {
		var lastErr error
		for i := 0; i < 3; i++ {
			if opCtx.Err() != nil {
				done <- opCtx.Err()
				return
			}
			token, err = retTokenFn(getTokenParams)
			if err != nil {
				lastErr = err
				continue // 如果失败则重试
			}
			done <- nil
			return
		}
		done <- fmt.Errorf("RetToken failed after 3 attempts (host=%s): %w", host, lastErr)
	}()

	select {
	case err := <-done:
		if err == nil {
			return token, nil
		}
		return "", err
	case <-opCtx.Done():
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("RetToken timed out after 10s (host=%s)", host)
	}
}
