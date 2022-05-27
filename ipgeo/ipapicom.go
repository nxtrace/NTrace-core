package ipgeo

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
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
	body, _ := ioutil.ReadAll(content.Body)
	res := gjson.ParseBytes(body)

	if res.Get("country").String() == "" {
		os.Exit(1)
	}
	log.Println("ip-api.com 正在使用")
	return &IPGeoData{
		Asnumber: strings.Split(res.Get("as").String(), " ")[0],
		Country:  res.Get("country").String(),
		City:     res.Get("city").String(),
		Prov:     res.Get("regionName").String(),
		Isp:      res.Get("isp").String(),
	}, nil
}
