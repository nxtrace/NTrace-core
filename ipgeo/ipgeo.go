package ipgeo

import (
	"strings"
	"time"
)

type IPGeoData struct {
	IP        string              `json:"ip"`
	Asnumber  string              `json:"asnumber"`
	Country   string              `json:"country"`
	CountryEn string              `json:"country_en"`
	Prov      string              `json:"prov"`
	ProvEn    string              `json:"prov_en"`
	City      string              `json:"city"`
	CityEn    string              `json:"city_en"`
	District  string              `json:"district"`
	Owner     string              `json:"owner"`
	Isp       string              `json:"isp"`
	Domain    string              `json:"domain"`
	Whois     string              `json:"whois"`
	Lat       float64             `json:"lat"`
	Lng       float64             `json:"lng"`
	Prefix    string              `json:"prefix"`
	Router    map[string][]string `json:"router"`
	Source    string              `json:"source"`
}

type Source = func(ip string, timeout time.Duration, lang string, maptrace bool) (*IPGeoData, error)

func GetSource(s string) Source {
	switch strings.ToUpper(s) {
	case "DN42":
		return DN42
	case "LEOMOEAPI":
		return LeoIP
	case "IP.SB":
		return IPSB
	case "IPINSIGHT":
		return IPInSight
	case "IPAPI.COM":
		return IPApiCom
	case "IP-API.COM":
		return IPApiCom
	case "IPINFO":
		return IPInfo
	case "IP2REGION":
		return IP2Region
	case "IPINFOLOCAL":
		return IPInfoLocal
	case "CHUNZHEN":
		return Chunzhen
	case "DISABLE-GEOIP":
		return disableGeoIP
	default:
		return LeoIP
	}
}

func disableGeoIP(string, time.Duration, string, bool) (*IPGeoData, error) {
	return &IPGeoData{}, nil
}
