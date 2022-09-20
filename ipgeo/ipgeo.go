package ipgeo

import (
	"strings"
)

type IPGeoData struct {
	Asnumber string
	Country  string
	Prov     string
	City     string
	District string
	Owner    string
	Isp      string
	Whois    string
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
