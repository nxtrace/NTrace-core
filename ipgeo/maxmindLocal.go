package ipgeo

import (
	"fmt"
	"net"
	"time"

	"github.com/oschwald/geoip2-golang"
)

const (
	maxmindASNDBPath  = "/usr/local/share/nexttrace/GeoLite2-ASN.mmdb"
	maxmindCityDBPath = "/usr/local/share/nexttrace/GeoLite2-City.mmdb"
)

func MaxMindLocal(ip string, _ time.Duration, lang string, _ bool) (*IPGeoData, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, fmt.Errorf("maxmindLocal: invalid IP address: %s", ip)
	}

	asnDB, err := geoip2.Open(maxmindASNDBPath)
	if err != nil {
		return nil, fmt.Errorf("maxmindLocal: cannot open ASN database: %w", err)
	}
	defer asnDB.Close()

	cityDB, err := geoip2.Open(maxmindCityDBPath)
	if err != nil {
		return nil, fmt.Errorf("maxmindLocal: cannot open City database: %w", err)
	}
	defer cityDB.Close()

	asnRecord, err := asnDB.ASN(parsedIP)
	if err != nil {
		return nil, fmt.Errorf("maxmindLocal: ASN lookup failed: %w", err)
	}

	cityRecord, err := cityDB.City(parsedIP)
	if err != nil {
		return nil, fmt.Errorf("maxmindLocal: City lookup failed: %w", err)
	}

	asnumber := fmt.Sprintf("AS%d", asnRecord.AutonomousSystemNumber)
	owner := asnRecord.AutonomousSystemOrganization

	country := cityRecord.Country.IsoCode
	city := ""
	prov := ""
	district := ""

	if lang == "cn" {
		if name, ok := cityRecord.City.Names["zh-CN"]; ok {
			city = name
		} else {
			city = cityRecord.City.Names["en"]
		}
		if len(cityRecord.Subdivisions) > 0 {
			if name, ok := cityRecord.Subdivisions[0].Names["zh-CN"]; ok {
				prov = name
			} else {
				prov = cityRecord.Subdivisions[0].Names["en"]
			}
		}
	} else {
		city = cityRecord.City.Names["en"]
		if len(cityRecord.Subdivisions) > 0 {
			prov = cityRecord.Subdivisions[0].Names["en"]
		}
	}

	lat := cityRecord.Location.Latitude
	lng := cityRecord.Location.Longitude

	return &IPGeoData{
		Asnumber: asnumber,
		Country:  country,
		City:     city,
		Prov:     prov,
		District: district,
		Owner:    owner,
		Lat:      lat,
		Lng:      lng,
	}, nil
}
