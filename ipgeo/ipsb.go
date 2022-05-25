package ipgeo

import (
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/tidwall/gjson"
)

func IPSB(ip string) (*IPGeoData, error) {
	url := "https://api.ip.sb/geoip/" + ip
	client := &http.Client{
		// 2 秒超时
		Timeout: 2 * time.Second,
	}
	req, _ := http.NewRequest("GET", url, nil)
	// 设置 UA，ip.sb 默认禁止 go-client User-Agent 的 api 请求
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:100.0) Gecko/20100101 Firefox/100.0")
	content, err := client.Do(req)
	if err != nil {
		log.Println("api.ip.sb 请求超时(2s)，请切换其他API使用")
		return nil, err
	}
	body, _ := ioutil.ReadAll(content.Body)
	res := gjson.ParseBytes(body)

	return &IPGeoData{
		Asnumber: res.Get("asn").String(),
		Country:  res.Get("country").String(),
		City:     res.Get("city").String(),
		Prov:     res.Get("region").String(),
		Isp:      res.Get("isp").String(),
	}, nil
}
