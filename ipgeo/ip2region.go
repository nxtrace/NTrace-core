package ipgeo

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/lionsoul2014/ip2region/v1.0/binding/golang/ip2region"
)

const (
	ipDataBasePath = "./ip2region.db"
	defaultDownURL = "1"
	originURL      = "https://mirror.ghproxy.com/?q=https://github.com/bqf9979/ip2region/blob/master/data/ip2region.db?raw=true"
)

func downloadDataBase() error {
	fmt.Println("Downloading DataBase...")
	resp, err := http.Get(originURL)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic(err)
		}
	}(resp.Body)

	// Create the file
	out, err := os.Create(ipDataBasePath)
	if err != nil {
		return err
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {
			panic(err)
		}
	}(out)

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func IP2Region(ip string, _ time.Duration, _ string, _ bool) (*IPGeoData, error) {
	if _, err := os.Stat(ipDataBasePath); os.IsNotExist(err) {
		if err = downloadDataBase(); err != nil {
			panic("Download Failed!")
		}
	}
	region, err := ip2region.New(ipDataBasePath)
	if err != nil {
		panic("Cannot find ip2region.db")
	}
	defer region.Close()
	info, searchErr := region.MemorySearch(ip)
	if searchErr != nil {
		return &IPGeoData{}, errors.New("no results")
	}

	if info.Country == "0" {
		info.Country = ""
	}

	if info.Province == "0" {
		info.Province = ""
	}

	if info.City == "0" {
		info.City = ""
	}

	if info.ISP == "0" {
		info.ISP = ""
	}

	return &IPGeoData{
		Owner:   info.ISP,
		Country: info.Country,
		Prov:    info.Province,
		City:    info.City,
	}, nil
}
