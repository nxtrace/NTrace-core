package util

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/fatih/color"
)

type ResponseInfo struct {
	IP      string
	Latency string
	Content string
}

var (
	results = make(chan ResponseInfo)
	timeout = 5 * time.Second
)
var FastIpCache = ""

// SuppressFastIPOutput 为 true 时，GetFastIP 即使 enableOutput=true 也不打印彩色输出。
// MTR 模式在进入备用屏前设置此标志，避免污染主终端历史。
var SuppressFastIPOutput bool

func GetFastIP(domain string, port string, enableOutput bool) string {
	proxyUrl := GetProxy()
	if proxyUrl != nil {
		return "api.nxtrace.org"
	}
	if FastIpCache != "" {
		return FastIpCache
	}

	var ips []net.IP
	var err error
	if domain == "api.nxtrace.org" {
		ips, err = net.LookupIP("api.nxtrace.org")
	} else {
		ips, err = net.LookupIP(domain)
	}

	if err != nil {
		log.Println("DNS resolution failed, please check your system DNS Settings")
	}

	if len(ips) == 0 {
		// 添加默认IP 45.88.195.154 2605:52c0:2:954:114:514:1919:810
		ips = append(ips, net.ParseIP("45.88.195.154"))
		ips = append(ips, net.ParseIP("2605:52c0:2:954:114:514:1919:810"))
	}

	for _, ip := range ips {
		go checkLatency(domain, ip.String(), port)
	}

	var result ResponseInfo

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	select {
	case result = <-results:
		// 正常返回结果
	case <-time.After(timeout):
		log.Println("IP connection has been timeout(5s), please check your network")
	case <-sigChan: // 响应中断信号
		log.Println("Program interrupted by user")
		os.Exit(0)
	}

	//if len(ips) > 0 {
	if enableOutput && !SuppressFastIPOutput {
		_, _ = fmt.Fprintf(color.Output, "%s preferred API IP - %s - %s - %s",
			color.New(color.FgWhite, color.Bold).Sprintf("[NextTrace API]"),
			color.New(color.FgGreen, color.Bold).Sprintf("%s", result.IP),
			color.New(color.FgCyan, color.Bold).Sprintf("%sms", result.Latency),
			color.New(color.FgGreen, color.Bold).Sprintf("%s", result.Content),
		)
	}
	//}

	//有些时候真的啥都不通，还是挑一个顶上吧
	if result.IP == "" {
		result.IP = "45.88.195.154"
	}

	FastIpCache = result.IP
	return result.IP
}

func checkLatency(domain string, ip string, port string) {
	start := time.Now()
	if !strings.Contains(ip, ".") {
		ip = "[" + ip + "]"
	}

	// 自定义DialContext以使用指定的IP连接
	transport := &http.Transport{
		//DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		//	return net.DialTimeout(network, addr, 1*time.Second)
		//},
		TLSClientConfig: &tls.Config{
			ServerName: domain,
		},
		TLSHandshakeTimeout: timeout,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	//此处虽然是 https://domain/ 但是实际上会使用指定的IP连接
	req, err := http.NewRequest("GET", "https://"+ip+":"+port+"/", nil)
	if err != nil {
		// !!! 此处不要给results返回任何值
		//results <- ResponseInfo{IP: ip, Latency: "error", Content: ""}
		return
	}
	req.Host = domain
	req.Header.Add("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		//results <- ResponseInfo{IP: ip, Latency: "error", Content: ""}
		return
	}
	if resp == nil || resp.Body == nil {
		// 防止后续对 nil Body 的读写导致 panic
		return
	}
	defer func() {
		// 明确忽略关闭时的错误，HTTP 客户端此时已经读完正文
		_ = resp.Body.Close()
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		//results <- ResponseInfo{IP: ip, Latency: "error", Content: ""}
		return
	}
	bodyString := string(bodyBytes)

	latency := fmt.Sprintf("%.2f", float64(time.Since(start))/float64(time.Millisecond))
	results <- ResponseInfo{IP: ip, Latency: latency, Content: bodyString}
}
