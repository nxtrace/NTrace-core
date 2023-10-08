package util

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
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
)
var FastIpCache = ""

func GetFastIP(domain string, port string, enableOutput bool) string {
	proxyUrl := GetProxy()
	if proxyUrl != nil {
		return "api.leo.moe"
	}
	if FastIpCache != "" {
		return FastIpCache
	}

	ips, err := net.LookupIP(domain)
	if err != nil {
		log.Fatal("DNS resolution failed, please check your system DNS Settings")
	}

	for _, ip := range ips {
		go checkLatency(domain, ip.String(), port)
	}

	var result ResponseInfo

	select {
	case result = <-results:
	case <-time.After(1 * time.Second):
		log.Fatal("IP connection has been timeout, please check your network")

	}

	if len(ips) > 0 {
		if enableOutput {
			_, _ = fmt.Fprintf(color.Output, "%s prefered API IP - %s - %s - %s",
				color.New(color.FgWhite, color.Bold).Sprintf("[NextTrace API]"),
				color.New(color.FgGreen, color.Bold).Sprintf("%s", result.IP),
				color.New(color.FgCyan, color.Bold).Sprintf("%sms", result.Latency),
				color.New(color.FgGreen, color.Bold).Sprintf("%s", result.Content),
			)
		}
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
		TLSHandshakeTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   2 * time.Second,
	}

	//此处虽然是 https://domain/ 但是实际上会使用指定的IP连接
	req, err := http.NewRequest("GET", "https://"+ip+":"+port+"/", nil)
	if err != nil {
		results <- ResponseInfo{IP: ip, Latency: "error", Content: ""}
		return
	}
	req.Host = domain
	resp, err := client.Do(req)
	if err != nil {
		results <- ResponseInfo{IP: ip, Latency: "error", Content: ""}
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		results <- ResponseInfo{IP: ip, Latency: "error", Content: ""}
		return
	}
	bodyString := string(bodyBytes)

	latency := fmt.Sprintf("%.2f", float64(time.Since(start))/float64(time.Millisecond))
	results <- ResponseInfo{IP: ip, Latency: latency, Content: bodyString}
}
