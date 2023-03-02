package ipgeo

import (
	"io/ioutil"
	"net/http"
	"strings"

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
	country = res.Get("country").String()
	if res.Get("country").String() == "HK" || res.Get("country").String() == "TW" {
		country = "CN"
	}

	return &IPGeoData{
		Asnumber: strings.Fields(strings.TrimPrefix(res.Get("org").String(), "AS"))[0],
		Country:  country,
		City:     res.Get("city").String(),
		Prov:     res.Get("region").String(),
		Owner:    res.Get("asn").Get("domain").String(),
	}, nil
}
