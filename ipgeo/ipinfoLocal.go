package ipgeo

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang"

	"github.com/nxtrace/NTrace-core/util"
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
func getIPInfoLocalPath() error {
	if ipinfoDataBasePath != "" {
		return nil
	}
	// NEXTTRACE_IPINFOLOCALPATH
	path := util.GetEnvDefault("NEXTTRACE_IPINFOLOCALPATH", "")
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			ipinfoDataBasePath = path
			return nil
		}
		return errors.New("NEXTTRACE_IPINFOLOCALPATH is set but the file does not exist")
	}
	var folders []string
	// current folder
	if cur, err := os.Getwd(); err == nil {
		folders = append(folders, cur+string(filepath.Separator))
	}
	// exeutable folder
	if exe, err := os.Executable(); err == nil {
		folders = append(folders, filepath.Dir(exe)+string(filepath.Separator))
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
		return nil, fmt.Errorf("ipinfoLocal: cannot find ipinfoLocal.mmdb: %w", err)
	}
	region, err := maxminddb.Open(ipinfoDataBasePath)
	if err != nil {
		return nil, fmt.Errorf("ipinfoLocal: cannot open %s: %w", ipinfoDataBasePath, err)
	}
	defer func(region *maxminddb.Reader) {
		_ = region.Close()
	}(region)
	var record interface{}
	searchErr := region.Lookup(net.ParseIP(ip), &record)
	if searchErr != nil {
		return &IPGeoData{}, errors.New("no results")
	}
	recordMap, ok := record.(map[string]interface{})
	if !ok {
		return &IPGeoData{}, errors.New("ipinfoLocal: unexpected record format")
	}
	countryName, _ := recordMap["country_name"].(string)
	countryCode, _ := recordMap["country"].(string)
	prov := ""
	if countryCode == "HK" {
		countryName = "China"
		prov = "Hong Kong"
	}
	if countryCode == "TW" {
		countryName = "China"
		prov = "Taiwan"
	}
	asnStr, _ := recordMap["asn"].(string)
	asName, _ := recordMap["as_name"].(string)
	return &IPGeoData{
		Asnumber: strings.TrimPrefix(asnStr, "AS"),
		Country:  countryName,
		City:     "",
		Prov:     prov,
		Owner:    asName,
	}, nil
}
