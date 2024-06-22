package ipgeo

import (
	"errors"
	"github.com/nxtrace/NTrace-core/util"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/tidwall/gjson"
)

func IPApiCom(ip string, timeout time.Duration, _ string, _ bool) (*IPGeoData, error) {
	url := token.BaseOrDefault("http://ip-api.com/json/") + ip + "?fields=status,message,country,regionName,city,isp,district,as,lat,lon"
	client := &http.Client{
		// 2 秒超时
		Timeout: timeout,
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
	var country = res.Get("country").String()
	var prov = res.Get("region").String()
	var city = res.Get("city").String()
	var district = res.Get("district").String()
	if util.StringInSlice(country, []string{"Hong Kong", "Taiwan", "Macao"}) {
		district = prov + " " + city + " " + district
		city = country
		prov = ""
		country = "China"
	}
	lat, _ := strconv.ParseFloat(res.Get("lat").String(), 32)
	lng, _ := strconv.ParseFloat(res.Get("lon").String(), 32)

	return &IPGeoData{
		Asnumber: re.FindString(res.Get("as").String()),
		Country:  country,
		City:     city,
		Prov:     prov,
		District: district,
		Owner:    res.Get("isp").String(),
		Lat:      lat,
		Lng:      lng,
	}, nil
}
