package tracemap

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/nxtrace/NTrace-core/util"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func GetMapUrl(r string) (string, error) {
	host, port := util.GetHostAndPort()
	var fastIp string
	// 如果 host 是一个 IP 使用默认域名
	if valid := net.ParseIP(host); valid != nil {
		fastIp = host
		if len(strings.Split(fastIp, ":")) > 1 {
			fastIp = "[" + fastIp + "]"
		}
		host = "origin-fallback.nxtrace.org"
	} else {
		// 默认配置完成，开始寻找最优 IP
		fastIp = util.GetFastIP(host, port, false)
	}
	u := url.URL{Scheme: "https", Host: fastIp + ":" + port, Path: "/tracemap/api"}
	tracemapUrl := u.String()

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				ServerName: host,
			},
		},
	}
	proxyUrl := util.GetProxy()
	if proxyUrl != nil {
		client.Transport.(*http.Transport).Proxy = http.ProxyURL(proxyUrl)
	}
	req, err := http.NewRequest("POST", tracemapUrl, strings.NewReader(r))
	if err != nil {
		return "", errors.New("an issue occurred while connecting to the tracemap API")
	}
	req.Header.Add("User-Agent", util.UserAgent)
	req.Host = host
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("an issue occurred while connecting to the tracemap API")
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New("an issue occurred while connecting to the tracemap API")
	}
	return string(body), nil
}

func PrintMapUrl(r string) {
	_, err := fmt.Fprintf(color.Output, "%s %s\n",
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "MapTrace URL:"),
		color.New(color.FgBlue, color.Bold).Sprintf("%s", r),
	)
	if err != nil {
		return
	}
}
