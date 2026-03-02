package ipgeo

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/tidwall/gjson"

	"github.com/nxtrace/NTrace-core/util"
)

func IPSB(ip string, timeout time.Duration, _ string, _ bool) (*IPGeoData, error) {
	url := token.BaseOrDefault("https://api.ip.sb/geoip/") + ip
	client := util.NewGeoHTTPClient(timeout)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("ip.sb: failed to create request: %w", err)
	}
	// 设置 UA，ip.sb 默认禁止 go-client User-Agent 的 api 请求
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:100.0) Gecko/20100101 Firefox/100.0")
	content, err := client.Do(req)
	if err != nil {
		log.Println("api.ip.sb 请求超时(2s)，请切换其他API使用")
		return nil, err
	}
	defer content.Body.Close()
	body, err := io.ReadAll(content.Body)
	if err != nil {
		return nil, fmt.Errorf("ip.sb: failed to read response: %w", err)
	}
	res := gjson.ParseBytes(body)

	if res.Get("country").String() == "" {
		// 什么都拿不到，证明被Cloudflare风控了
		return nil, fmt.Errorf("ip.sb: empty response, possibly blocked by Cloudflare")
	}

	return &IPGeoData{
		Asnumber: res.Get("asn").String(),
		Country:  res.Get("country").String(),
		City:     res.Get("city").String(),
		Prov:     res.Get("region").String(),
		Owner:    res.Get("isp").String(),
	}, nil
}
