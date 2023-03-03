package ipgeo

import (
	"errors"
	"github.com/oschwald/maxminddb-golang"
	"net"
	"os"
	"strings"
)

const (
	ipinfoDataBasePath = "./ipinfoLocal.mmdb"
)

func IPInfoLocal(ip string) (*IPGeoData, error) {
	if _, err := os.Stat(ipinfoDataBasePath); os.IsNotExist(err) {
		panic("Cannot find ipinfoLocal.mmdb")
	}
	region, err := maxminddb.Open(ipinfoDataBasePath)
	if err != nil {
		panic("Cannot find ipinfoLocal.mmdb")
	}
	defer func(region *maxminddb.Reader) {
		err := region.Close()
		if err != nil {
			panic(err)
		}
	}(region)
	var record interface{}
	searchErr := region.Lookup(net.ParseIP(ip), &record)
	if searchErr != nil {
		return &IPGeoData{}, errors.New("no results")
	}
	recordMap := record.(map[string]interface{})
	country_name := recordMap["country_name"].(string)
	prov := ""
	if recordMap["country"].(string) == "HK" {
		country_name = "China"
		prov = "Hong Kong"
	}
	if recordMap["country"].(string) == "TW" {
		country_name = "China"
		prov = "Taiwan"
	}
	return &IPGeoData{
		Asnumber: strings.TrimPrefix(recordMap["asn"].(string), "AS"),
		Country:  country_name,
		City:     "",
		Prov:     prov,
		Owner:    recordMap["as_name"].(string),
	}, nil
}
