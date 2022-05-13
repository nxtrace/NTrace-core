package ipgeo

import (
	"io/ioutil"
	"net/http"

	"github.com/tidwall/gjson"
)

func IPInfo(ip string) (*IPGeoData, error) {

	resp, err := http.Get("https://ipinfo.io/" + ip + "?token=42764a944dabd0")
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
		Country: res.Get("country").String(),
		City:    res.Get("city").String(),
		Prov:    res.Get("region").String(),
	}, nil
}
