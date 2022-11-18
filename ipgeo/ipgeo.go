package ipgeo

import (
	"strings"
)

type IPGeoData struct {
	IP       string              `json:"ip"`
	Asnumber string              `json:"asnumber"`
	Country  string              `json:"country"`
	Prov     string              `json:"prov"`
	City     string              `json:"city"`
	District string              `json:"district"`
	Owner    string              `json:"owner"`
	Isp      string              `json:"isp"`
	Domain   string              `json:"domain"`
	Whois    string              `json:"whois"`
	Prefix   string              `json:"prefix"`
	Router   map[string][]string `json:"router"`
	Source   string              `json:"source"`
}

type Source = func(ip string) (*IPGeoData, error)

func GetSource(s string) Source {
	switch strings.ToUpper(s) {
	case "LEOMOEAPI":
		return LeoIP
	case "IP.SB":
		return IPSB
	case "IPINSIGHT":
		return IPInSight
	case "IPAPI.COM":
		return IPApiCom
	case "IPINFO":
		return IPInfo
	case "IP2REGION":
		return IP2Region
	default:
		return LeoIP
	}
}
