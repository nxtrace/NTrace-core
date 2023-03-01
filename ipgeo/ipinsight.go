package ipgeo

import (
	"io/ioutil"
	"net/http"

	"github.com/tidwall/gjson"
)

func IPInSight(ip string) (*IPGeoData, error) {
	resp, err := http.Get("https://api.ipinsight.io/query?ip=" + ip + "?token=" + token.ipinsight)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
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
