package ipgeo

import (
	"github.com/tidwall/gjson"
	"io/ioutil"
	"net/http"
)

func LeoIP(ip string) (*IPGeoData, error) {
	resp, err := http.Get("https://api.leo.moe/ip/?ip=" + ip)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	res := gjson.ParseBytes(body)
	return &IPGeoData{
		Asnumber: res.Get("asnumber").String(),
		Country:  res.Get("country").String(),
		Prov:     res.Get("prov").String(),
		City:     res.Get("city").String(),
		District: res.Get("district").String(),
		Owner:    res.Get("owner").String(),
		Isp:      res.Get("isp").String(),
	}, nil
}
