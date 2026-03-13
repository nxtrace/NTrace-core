package ipgeo

import (
	"net"
)

type cidrFilterRule struct {
	cidr  string
	whois string
}

var reservedCIDRRules = []cidrFilterRule{
	{cidr: "0.0.0.0/8", whois: "RFC1122"},
	{cidr: "100.64.0.0/10", whois: "RFC6598"},
	{cidr: "127.0.0.0/8", whois: "RFC1122"},
	{cidr: "169.254.0.0/16", whois: "RFC3927"},
	{cidr: "192.0.0.0/24", whois: "RFC6890"},
	{cidr: "192.0.2.0/24", whois: "RFC5737"},
	{cidr: "192.88.99.0/24", whois: "RFC3068"},
	{cidr: "198.18.0.0/15", whois: "RFC2544"},
	{cidr: "198.51.100.0/24", whois: "RFC5737"},
	{cidr: "203.0.113.0/24", whois: "RFC5737"},
	{cidr: "224.0.0.0/4", whois: "RFC5771"},
	{cidr: "255.255.255.255/32", whois: "RFC0919"},
	{cidr: "240.0.0.0/4", whois: "RFC1112"},
	{cidr: "fe80::/10", whois: "RFC4291"},
	{cidr: "ff00::/8", whois: "RFC4291"},
	{cidr: "fec0::/10", whois: "RFC3879"},
	{cidr: "fe00::/9", whois: "RFC4291"},
	{cidr: "64:ff9b::/96", whois: "RFC6052"},
	{cidr: "0::/96", whois: "RFC4291"},
	{cidr: "64:ff9b:1::/48", whois: "RFC6052"},
	{cidr: "2001:db8::/32", whois: "RFC3849"},
	{cidr: "2002::/16", whois: "RFC3056"},
}

var dodCIDRRules = []cidrFilterRule{
	{cidr: "6.0.0.0/8", whois: "DOD"},
	{cidr: "7.0.0.0/8", whois: "DOD"},
	{cidr: "11.0.0.0/8", whois: "DOD"},
	{cidr: "21.0.0.0/8", whois: "DOD"},
	{cidr: "22.0.0.0/8", whois: "DOD"},
	{cidr: "26.0.0.0/8", whois: "DOD"},
	{cidr: "28.0.0.0/8", whois: "DOD"},
	{cidr: "29.0.0.0/8", whois: "DOD"},
	{cidr: "30.0.0.0/8", whois: "DOD"},
	{cidr: "33.0.0.0/8", whois: "DOD"},
	{cidr: "55.0.0.0/8", whois: "DOD"},
	{cidr: "214.0.0.0/8", whois: "DOD"},
	{cidr: "215.0.0.0/8", whois: "DOD"},
}

func cidrRangeContains(cidrRange string, checkIP string) bool {
	_, ipNet, err := net.ParseCIDR(cidrRange)
	if err != nil {
		return false
	}
	secondIP := net.ParseIP(checkIP)
	return ipNet.Contains(secondIP)
}

func matchCIDRFilterRule(ip string, rules []cidrFilterRule) (string, bool) {
	for _, rule := range rules {
		if cidrRangeContains(rule.cidr, ip) {
			return rule.whois, true
		}
	}
	return "", false
}

func classifyPrivateIP(parsedIP net.IP, rawIP string) (string, bool) {
	if parsedIP == nil || !parsedIP.IsPrivate() {
		return "", false
	}
	if cidrRangeContains("fc00::/7", rawIP) {
		return "RFC4193", true
	}
	return "RFC1918", true
}

func isInvalidScopedIPv6(parsedIP net.IP, rawIP string) bool {
	return parsedIP != nil && parsedIP.To4() == nil && !cidrRangeContains("2000::/3", rawIP)
}

// Filter 被选到的返回 geodata, true  否则返回 nil, false
func Filter(ip string) (*IPGeoData, bool) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, false
	}

	if whois, ok := matchCIDRFilterRule(ip, reservedCIDRRules); ok {
		return &IPGeoData{Whois: whois}, true
	}
	if whois, ok := classifyPrivateIP(parsedIP, ip); ok {
		return &IPGeoData{Whois: whois}, true
	}
	if whois, ok := matchCIDRFilterRule(ip, dodCIDRRules); ok {
		return &IPGeoData{Whois: whois}, true
	}
	if isInvalidScopedIPv6(parsedIP, ip) {
		return &IPGeoData{Whois: "INVALID"}, true
	}

	return nil, false
}
