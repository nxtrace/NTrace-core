package dn42

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

type PtrRow struct {
	IATACode string
	LtdCode  string
	Region   string
	City     string
}

func matchesPattern(prefix string, s string) bool {
	pattern := fmt.Sprintf(`^(.*[-.\d]|^)%s[-.\d].*$`, prefix)

	r, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Println("Invalid regular expression:", err)
		return false
	}

	return r.MatchString(s)
}

func FindPtrRecord(ptr string) (PtrRow, error) {
	path := viper.Get("ptrPath").(string)
	f, err := os.Open(path)
	if err != nil {
		return PtrRow{}, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return PtrRow{}, err
	}
	// 转小写
	ptr = strings.ToLower(ptr)
	// 先查城市名
	for _, row := range rows {
		city := row[3]
		if city == "" {
			continue
		}
		city = strings.ReplaceAll(city, " ", "")
		city = strings.ToLower(city)

		if matchesPattern(city, ptr) {
			return PtrRow{
				LtdCode: row[1],
				Region:  row[2],
				City:    row[3],
			}, nil
		}
	}
	// 查 IATA Code
	for _, row := range rows {
		iata := row[0]
		if iata == "" {
			continue
		}
		iata = strings.ToLower(iata)
		if matchesPattern(iata, ptr) {
			return PtrRow{
				IATACode: iata,
				LtdCode:  row[1],
				Region:   row[2],
				City:     row[3],
			}, nil
		}
	}

	return PtrRow{}, errors.New("ptr not found")
}
