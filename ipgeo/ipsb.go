package ipgeo

import (
	"io/ioutil"
	"net/http"

	"github.com/tidwall/gjson"
)

func IPSB(ip string) (*IPGeoData, error) {
	resp, err := http.Get("https://api.ip.sb/geoip/" + ip)
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
		Asnumber: res.Get("asn").String(),
		Country:  res.Get("country").String(),
		City:     res.Get("city").String(),
		Prov:     res.Get("region").String(),
		Isp:      res.Get("isp").String(),
	}, nil
}
