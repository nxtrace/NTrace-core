package ipgeo

import (
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/tidwall/gjson"
)

func IPSB(ip string, timeout time.Duration, _ string, _ bool) (*IPGeoData, error) {
	url := token.BaseOrDefault("https://api.ip.sb/geoip/") + ip
	client := &http.Client{
		// 2 秒超时
		Timeout: timeout,
	}
	req, _ := http.NewRequest("GET", url, nil)
	// 设置 UA，ip.sb 默认禁止 go-client User-Agent 的 api 请求
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:100.0) Gecko/20100101 Firefox/100.0")
	content, err := client.Do(req)
	if err != nil {
		log.Println("api.ip.sb 请求超时(2s)，请切换其他API使用")
		return nil, err
	}
	body, _ := io.ReadAll(content.Body)
	res := gjson.ParseBytes(body)

	if res.Get("country").String() == "" {
		// 什么都拿不到，证明被Cloudflare风控了
		os.Exit(1)
	}

	return &IPGeoData{
		Asnumber: res.Get("asn").String(),
		Country:  res.Get("country").String(),
		City:     res.Get("city").String(),
		Prov:     res.Get("region").String(),
		Owner:    res.Get("isp").String(),
	}, nil
}
