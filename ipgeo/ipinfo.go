package ipgeo

import (
	"io/ioutil"
	"net/http"

	"github.com/tidwall/gjson"
)

func IPInfo(ip string) (*IPGeoData, error) {
	resp, err := http.Get("https://ipinfo.io/" + ip + "?token=" + token.ipinfo)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	res := gjson.ParseBytes(body)

	var country string

	if res.Get("country").String() == "HK" || res.Get("country").String() == "TW" {
		country = "CN"
	}

	return &IPGeoData{
		Asnumber: res.Get("asn").Get("asn").String(),
		Country:  country,
		City:     res.Get("city").String(),
		Prov:     res.Get("region").String(),
		Isp:      res.Get("asn").Get("domain").String(),
	}, nil
}
