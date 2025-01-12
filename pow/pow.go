package pow

import (
	"fmt"
	"github.com/nxtrace/NTrace-core/util"
	"github.com/tsosunchia/powclient"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
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
		done  = make(chan bool)
	)
	// 在 goroutine 中处理阻塞调用
	go func() {
		for i := 0; i < 3; i++ {
			token, err = powclient.RetToken(getTokenParams)
			if err != nil {
				continue // 如果失败则重试
			}
			done <- true // 成功后通知主线程
			return
		}
		done <- false // 失败后通知主线程
	}()

	select {
	case <-sigChan: // 监听中断信号
		return "", fmt.Errorf("Program interrupted by user ") // 添加返回值
	case success := <-done: // 等待 goroutine 完成
		if success {
			return token, nil
		}
		return "", fmt.Errorf("RetToken failed 3 times, please try again later")
	case <-time.After(10 * time.Second): // 超时处理
		return "", fmt.Errorf("RetToken timed out(10s), please check your network") // 添加返回值
	}
}
