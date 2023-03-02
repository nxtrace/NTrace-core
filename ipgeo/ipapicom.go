package ipgeo

import (
	"errors"
	"io"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/tidwall/gjson"
)

func IPApiCom(ip string) (*IPGeoData, error) {
	url := "http://ip-api.com/json/" + ip + "?fields=status,message,country,regionName,city,isp,as"
	client := &http.Client{
		// 2 秒超时
		Timeout: 2 * time.Second,
	}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:100.0) Gecko/20100101 Firefox/100.0")
	content, err := client.Do(req)
	if err != nil {
		log.Println("ip-api.com 请求超时(2s)，请切换其他API使用")
		return nil, err
	}
	body, _ := io.ReadAll(content.Body)
	res := gjson.ParseBytes(body)

	if res.Get("status").String() != "success" {
		return &IPGeoData{}, errors.New("超过API阈值")
	}

	re := regexp.MustCompile("[0-9]+")

	return &IPGeoData{
		Asnumber: re.FindString(res.Get("as").String()),
		Country:  res.Get("country").String(),
		City:     res.Get("city").String(),
		Prov:     res.Get("regionName").String(),
		Owner:    res.Get("isp").String(),
	}, nil
}
