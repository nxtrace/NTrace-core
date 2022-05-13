package geodata

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/xgadget-lab/nexttrace/util"
)

type IPInSightData struct {
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

type IPSBData struct {
	Organization    string  `json:"organization"`
	Longitude       float64 `json:"longitude"`
	City            string  `json:"city"`
	Timezone        string  `json:"timezone"`
	Isp             string  `json:"isp"`
	Offset          int     `json:"offset"`
	Region          string  `json:"region"`
	Asn             int     `json:"asn"`
	AsnOrganization string  `json:"asn_organization"`
	Country         string  `json:"country"`
	IP              string  `json:"ip"`
	Latitude        float64 `json:"latitude"`
	PostalCode      string  `json:"postal_code"`
	ContinentCode   string  `json:"continent_code"`
	CountryCode     string  `json:"country_code"`
	RegionCode      string  `json:"region_code"`
}

type IPInfoData struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Loc      string `json:"loc"`
	Org      string `json:"org"`
	Postal   string `json:"postal"`
	Timezone string `json:"timezone"`
}

func GetIPGeoByIPInfo(ip string, c chan util.IPGeoData) {

	resp, err := http.Get("https://ipinfo.io/" + ip + "?token=42764a944dabd0")
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	iPInfoData := &IPInfoData{}
	err = json.Unmarshal(body, &iPInfoData)

	if err != nil {
		fmt.Println(err)
	}

	ipGeoData := util.IPGeoData{
		Country: iPInfoData.Country,
		City:    iPInfoData.City,
		Prov:    iPInfoData.Region}

	c <- ipGeoData
}

func GetIPGeoByIPSB(ip string, c chan util.IPGeoData) {
	resp, err := http.Get("https://api.ip.sb/geoip/" + ip)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	iPSBData := &IPSBData{}
	err = json.Unmarshal(body, &iPSBData)

	if err != nil {
		fmt.Println("您当前出口IP被IP.SB视为风控IP，请求被拒绝")
		c <- util.IPGeoData{}
	}

	ipGeoData := util.IPGeoData{
		Asnumber: strconv.Itoa(iPSBData.Asn),
		Isp:      iPSBData.Isp,
		Country:  iPSBData.Country,
		City:     iPSBData.City,
		Prov:     iPSBData.Region}

	c <- ipGeoData
}

func GetIPGeoByIPInsight(ip string, c chan util.IPGeoData) {

	resp, err := http.Get("https://ipinsight.io/query?ip=" + ip)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	iPInSightData := &IPInSightData{}
	err = json.Unmarshal(body, &iPInSightData)

	if err != nil {
		fmt.Println(err)
	}

	ipGeoData := util.IPGeoData{
		Country: iPInSightData.CountryName,
		City:    iPInSightData.CityName,
		Prov:    iPInSightData.RegionName}

	c <- ipGeoData
}

func GetIPGeo(ip string, c chan util.IPGeoData) {
	resp, err := http.Get("https://api.leo.moe/ip/?ip=" + ip)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	ipGeoData := util.IPGeoData{}
	err = json.Unmarshal(body, &ipGeoData)

	if err != nil {
		fmt.Println(err)
	}
	c <- ipGeoData
}
