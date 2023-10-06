package ipgeo

import (
	"encoding/json"
	"github.com/nxtrace/NTrace-core/util"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func Chunzhen(ip string, timeout time.Duration, _ string, _ bool) (*IPGeoData, error) {
	url := util.GetenvDefault("NEXTTRACE_CHUNZHENURL", "http://127.0.0.1:2060") + "?ip=" + ip
	client := &http.Client{
		// 2 秒超时
		Timeout: timeout,
	}
	req, _ := http.NewRequest("GET", url, nil)
	content, err := client.Do(req)
	if err != nil {
		log.Println("纯真 请求超时(2s)，请切换其他API使用")
		return &IPGeoData{}, err
	}
	body, _ := io.ReadAll(content.Body)

	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return &IPGeoData{}, err
	}
	city := data[ip].(map[string]interface{})["area"].(string)
	region := data[ip].(map[string]interface{})["country"].(string)
	var asn string
	if data[ip].(map[string]interface{})["asn"] != nil {
		asn = data[ip].(map[string]interface{})["asn"].(string)
	}
	// 判断是否前两个字为香港或台湾
	var country string
	provinces := []string{
		"北京",
		"天津",
		"河北",
		"山西",
		"内蒙古",
		"辽宁",
		"吉林",
		"黑龙江",
		"上海",
		"江苏",
		"浙江",
		"安徽",
		"福建",
		"江西",
		"山东",
		"河南",
		"湖北",
		"湖南",
		"广东",
		"广西",
		"海南",
		"重庆",
		"四川",
		"贵州",
		"云南",
		"西藏",
		"陕西",
		"甘肃",
		"青海",
		"宁夏",
		"新疆",
		"台湾",
		"香港",
		"澳门",
	}
	for _, province := range provinces {
		if strings.Contains(region, province) {
			country = "中国"
			city = region + city
			break
		}
	}
	if country == "" {
		country = region
	}
	return &IPGeoData{
		Asnumber: asn,
		Country:  country,
		City:     city,
	}, nil
}
