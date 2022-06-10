package ipgeo

import (
	"io/ioutil"
	"net/http"

	"github.com/tidwall/gjson"
)

func LeoIP(ip string) (*IPGeoData, error) {
	resp, err := http.Get("https://api.leo.moe/ip/?ip=" + ip + "&token=" + token.ipleo)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	res := gjson.ParseBytes(body)

	if res.Get("Message").String() != "" {
		return &IPGeoData{
			Prov: res.Get("Message").String(),
		}, nil
	}

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
