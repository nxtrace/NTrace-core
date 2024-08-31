package ipgeo

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	
	"github.com/nxtrace/NTrace-core/util"
	"github.com/oschwald/maxminddb-golang"
)

const (
	ipinfoDataBaseFilename = "ipinfoLocal.mmdb"
)

// Cache the path of the ipinfoLocal.mmdb file
var ipinfoDataBasePath = ""

// We will try to get the path of the ipinfoLocal.mmdb file in the following order:
// 1. Use the value of the environment variable NEXTTRACE_IPINFOLOCALPATH
// 2. Search in the current folder and the executable folder
// 3. Search in /usr/local/share/nexttrace/ and /usr/share/nexttrace/ (for Unix/Linux)
// If the file is found, the path will be stored in the ipinfoDataBasePath variable
func getIPInfoLocalPath() (error) {
	if ipinfoDataBasePath != "" {
		return nil
	}
	// NEXTTRACE_IPINFOLOCALPATH
	if util.EnvIPInfoLocalPath != "" {
		if _, err := os.Stat(util.EnvIPInfoLocalPath); err == nil {
			ipinfoDataBasePath = util.EnvIPInfoLocalPath
			return nil
		} else {
			return errors.New("NEXTTRACE_IPINFOLOCALPATH is set but the file does not exist")
		}
	}
	folders := []string{}
	// current folder
	if cur, err := os.Getwd(); err == nil {
		folders = append(folders, cur + string(filepath.Separator))
	}
	// exeutable folder
	if exe, err := os.Executable(); err == nil {
		folders = append(folders, filepath.Dir(exe) + string(filepath.Separator))
	}
	if runtime.GOOS != "windows" {
		folders = append(folders, "/usr/local/share/nexttrace/")
		folders = append(folders, "/usr/share/nexttrace/")
	}
	for _, folder := range folders {
		if _, err := os.Stat(folder + ipinfoDataBaseFilename); err == nil {
			ipinfoDataBasePath = folder + ipinfoDataBaseFilename
			return nil
		}
	}
	return errors.New("no ipinfoLocal.mmdb found")
}

func IPInfoLocal(ip string, _ time.Duration, _ string, _ bool) (*IPGeoData, error) {
	if err := getIPInfoLocalPath(); err != nil {
		panic("Cannot find ipinfoLocal.mmdb")
	}
	region, err := maxminddb.Open(ipinfoDataBasePath)
	if err != nil {
		panic("Cannot open ipinfoLocal.mmdb at " + ipinfoDataBasePath)
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
	countryName := recordMap["country_name"].(string)
	prov := ""
	if recordMap["country"].(string) == "HK" {
		countryName = "China"
		prov = "Hong Kong"
	}
	if recordMap["country"].(string) == "TW" {
		countryName = "China"
		prov = "Taiwan"
	}
	return &IPGeoData{
		Asnumber: strings.TrimPrefix(recordMap["asn"].(string), "AS"),
		Country:  countryName,
		City:     "",
		Prov:     prov,
		Owner:    recordMap["as_name"].(string),
	}, nil
}
