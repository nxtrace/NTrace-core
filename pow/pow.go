package pow

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tsosunchia/powclient"

	"github.com/nxtrace/NTrace-core/util"
)

const (
	baseURL = "/v3/challenge"
)

func GetToken(fastIp string, host string, port string) (string, error) {
	// 捕获中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	getTokenParams := powclient.NewGetTokenParams()
	u := url.URL{Scheme: "https", Host: fastIp + ":" + port, Path: baseURL}
	getTokenParams.BaseUrl = u.String()
	getTokenParams.SNI = host
	getTokenParams.Host = host
	getTokenParams.UserAgent = util.UserAgent
	proxyUrl := util.GetProxy()
	if proxyUrl != nil {
		getTokenParams.Proxy = proxyUrl
	}
	var (
		token string
		err   error
		done  = make(chan bool, 1)
		errCh = make(chan error, 1)
	)
	// 在 goroutine 中处理阻塞调用
	go func() {
		var lastErr error
		for i := 0; i < 3; i++ {
			token, err = powclient.RetToken(getTokenParams)
			if err != nil {
				lastErr = err
				continue // 如果失败则重试
			}
			done <- true // 成功后通知主线程
			return
		}
		// 三次都失败：先投递最后一次错误，再通知主线程失败
		errCh <- lastErr
		done <- false
	}()

	select {
	case <-sigChan: // 监听中断信号
		return "", fmt.Errorf("RetToken interrupted by user (host=%s)", host)
	case success := <-done: // 等待 goroutine 完成
		if success {
			return token, nil
		}
		lastErr := <-errCh
		// 不直接输出，错误上抛给上层
		return "", fmt.Errorf("RetToken failed after 3 attempts (host=%s): %w", host, lastErr)
	case <-time.After(10 * time.Second): // 超时处理
		return "", fmt.Errorf("RetToken timed out after 10s (host=%s)", host)
	}
}
