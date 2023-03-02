package ipgeo

import (
	"net"
)

func cidrRangeContains(cidrRange string, checkIP string) bool {
	_, ipNet, err := net.ParseCIDR(cidrRange)
	if err != nil {
		return false
	}
	secondIP := net.ParseIP(checkIP)
	return ipNet.Contains(secondIP)
}

// Filter 被选到的返回 geodata, true  否则返回 nil, false
func Filter(ip string) (*IPGeoData, bool) {
	//geodata := &IPGeoData{}
	asn := ""
	whois := ""
	isFiltered := false
	switch {
	//rfc1918
	case net.ParseIP(ip).IsPrivate():
		asn = ""
		whois = "RFC1918"
		isFiltered = true
	//IANA Reserved Address Space
	case cidrRangeContains("100.64.0.0/10", ip):
		asn = ""
		whois = "RFC6598"
		isFiltered = true
	case cidrRangeContains("198.18.0.0/15", ip):
		asn = ""
		whois = "RFC2544"
		isFiltered = true
	case cidrRangeContains("198.51.100.0/24", ip):
		fallthrough
	case cidrRangeContains("203.0.113.0/24", ip):
		asn = ""
		whois = "RFC5737"
		isFiltered = true
	case cidrRangeContains("240.0.0.0/4", ip):
		asn = ""
		whois = "RFC1112"
		isFiltered = true
	//Defense Information System Network
	case cidrRangeContains("6.0.0.0/8", ip):
		fallthrough
	case cidrRangeContains("7.0.0.0/8", ip):
		fallthrough
	case cidrRangeContains("11.0.0.0/8", ip):
		fallthrough
	case cidrRangeContains("21.0.0.0/8", ip):
		fallthrough
	case cidrRangeContains("22.0.0.0/8", ip):
		fallthrough
	case cidrRangeContains("26.0.0.0/8", ip):
		fallthrough
	case cidrRangeContains("28.0.0.0/8", ip):
		fallthrough
	case cidrRangeContains("29.0.0.0/8", ip):
		fallthrough
	case cidrRangeContains("30.0.0.0/8", ip):
		fallthrough
	case cidrRangeContains("33.0.0.0/8", ip):
		fallthrough
	case cidrRangeContains("55.0.0.0/8", ip):
		fallthrough
	case cidrRangeContains("214.0.0.0/8", ip):
		fallthrough
	case cidrRangeContains("215.0.0.0/8", ip):
		asn = ""
		whois = "DOD"
		isFiltered = true
	default:
	}
	if !isFiltered {
		return nil, false
	} else {
		return &IPGeoData{
			Asnumber: asn,
			Whois:    whois,
		}, true
	}
}
