package ipgeo

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type ipInSightData struct {
	IP              string  `json:"ip"`
	Version         string  `json:"version"`
	IsEuropeanUnion bool    `json:"is_european_union"`
	ContinentCode   string  `json:"continent_code"`
	IddCode         string  `json:"idd_code"`
	CountryCode     string  `json:"country_code"`
	CountryName     string  `json:"country_name"`
	RegionName      string  `json:"region_name"`
	CityName        string  `json:"city_name"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
}

func IPInSight(ip string) (*IPGeoData, error) {
	resp, err := http.Get("https://ipinsight.io/query?ip=" + ip)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	iPInSightData := &ipInSightData{}
	err = json.Unmarshal(body, &iPInSightData)
	if err != nil {
		return nil, err
	}

	return &IPGeoData{
		Country: iPInSightData.CountryName,
		City:    iPInSightData.CityName,
		Prov:    iPInSightData.RegionName,
	}, nil
}
