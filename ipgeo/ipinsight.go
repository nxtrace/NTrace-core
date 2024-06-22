package ipgeo

import (
	"io"
	"net/http"
	"time"

	"github.com/tidwall/gjson"
)

func IPInSight(ip string, timeout time.Duration, _ string, _ bool) (*IPGeoData, error) {
	client := &http.Client{
		// 2 秒超时
		Timeout: timeout,
	}
	resp, err := client.Get(token.BaseOrDefault("https://api.ipinsight.io/ip/") + ip + "?token=" + token.ipinsight)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	res := gjson.ParseBytes(body)

	return &IPGeoData{
		Country: res.Get("country_name").String(),
		City:    res.Get("city_name").String(),
		Prov:    res.Get("region_name").String(),
	}, nil
}
