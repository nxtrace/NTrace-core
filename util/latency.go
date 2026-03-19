package util

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

type ResponseInfo struct {
	IP      string
	Latency string
	Content string
}

var (
	timeout       = 5 * time.Second
	fastIPCacheMu sync.RWMutex
)
var FastIpCache = ""

var (
	fastIPLookupHostFn = LookupHostForGeo
	fastIPCheckLatency = checkLatencyWithContext
)

// FastIPMeta 存储 FastIP 节点的结构化元数据。
type FastIPMeta struct {
	IP       string // 节点 IP
	Latency  string // 延迟（ms 字符串）
	NodeName string // 节点名称（API 返回的 Content 去除前后空白）
}

// FastIPMetaCache 缓存最近一次 FastIP 探测返回的节点元数据。
var FastIPMetaCache FastIPMeta

// SuppressFastIPOutput 为 true 时，GetFastIP 即使 enableOutput=true 也不打印彩色输出。
// MTR 模式在进入备用屏前设置此标志，避免污染主终端历史。
var SuppressFastIPOutput bool

func GetFastIP(domain string, port string, enableOutput bool) string {
	ip, err := GetFastIPWithContext(context.Background(), domain, port, enableOutput)
	if err != nil {
		log.Printf("FastIP probe failed: %v", err)
		return defaultFastIP()
	}
	return ip
}

func GetFastIPCache() string {
	fastIPCacheMu.RLock()
	defer fastIPCacheMu.RUnlock()
	return FastIpCache
}

func GetFastIPMetaCache() FastIPMeta {
	fastIPCacheMu.RLock()
	defer fastIPCacheMu.RUnlock()
	return FastIPMetaCache
}

func SetFastIPCacheState(ip string, meta FastIPMeta) {
	fastIPCacheMu.Lock()
	FastIpCache = ip
	FastIPMetaCache = meta
	fastIPCacheMu.Unlock()
}

func GetFastIPWithContext(ctx context.Context, domain string, port string, enableOutput bool) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	proxyUrl := GetProxy()
	if proxyUrl != nil {
		return "api.nxtrace.org", nil
	}
	if cachedIP := GetFastIPCache(); cachedIP != "" {
		return cachedIP, nil
	}

	var ips []net.IP
	var err error
	lookupCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if domain == "api.nxtrace.org" {
		ips, err = fastIPLookupHostFn(lookupCtx, "api.nxtrace.org")
	} else {
		ips, err = fastIPLookupHostFn(lookupCtx, domain)
	}

	if err != nil {
		if lookupCtx.Err() != nil {
			return "", lookupCtx.Err()
		}
		log.Println("DNS resolution failed, please check your system DNS Settings")
	}

	if len(ips) == 0 {
		ips = defaultFastIPCandidates()
	}

	results := make(chan ResponseInfo, len(ips))
	for _, ip := range ips {
		go fastIPCheckLatency(ctx, domain, ip.String(), port, results)
	}

	var result ResponseInfo

	select {
	case result = <-results:
		// 正常返回结果
	case <-time.After(timeout):
		log.Println("IP connection has been timeout(5s), please check your network")
	case <-ctx.Done():
		return "", ctx.Err()
	}

	//有些时候真的啥都不通，还是挑一个顶上吧
	if result.IP == "" {
		result.IP = defaultFastIP()
	}

	meta := FastIPMeta{
		IP:       result.IP,
		Latency:  result.Latency,
		NodeName: strings.TrimSpace(result.Content),
	}
	SetFastIPCacheState(result.IP, meta)

	if enableOutput && !SuppressFastIPOutput {
		_, _ = fmt.Fprintf(color.Output, "%s preferred API IP - %s - %s - %s",
			color.New(color.FgWhite, color.Bold).Sprintf("[NextTrace API]"),
			color.New(color.FgGreen, color.Bold).Sprintf("%s", result.IP),
			color.New(color.FgCyan, color.Bold).Sprintf("%sms", result.Latency),
			color.New(color.FgGreen, color.Bold).Sprintf("%s", result.Content),
		)
	}

	return result.IP, nil
}

func defaultFastIPCandidates() []net.IP {
	return []net.IP{
		net.ParseIP("45.88.195.154"),
		net.ParseIP("2605:52c0:2:954:114:514:1919:810"),
	}
}

func defaultFastIP() string {
	return "45.88.195.154"
}

func checkLatencyWithContext(ctx context.Context, domain string, ip string, port string, results chan<- ResponseInfo) {
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
	req, err := http.NewRequestWithContext(ctx, "GET", "https://"+ip+":"+port+"/", nil)
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
	select {
	case results <- ResponseInfo{IP: ip, Latency: latency, Content: bodyString}:
	case <-ctx.Done():
	default:
	}
}
