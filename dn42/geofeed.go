package dn42

import (
	"encoding/csv"
	"fmt"
	"net"
	"os"
	"sort"

	"github.com/spf13/viper"
)

type GeoFeedRow struct {
	IPNet   *net.IPNet
	CIDR    string
	LtdCode string
	ISO3166 string
	City    string
	ASN     string
	IPWhois string
}

func GetGeoFeed(ip string) (GeoFeedRow, bool) {
	rows, err := ReadGeoFeed()
	if err != nil {
		// 无法加载 geofeed 数据，返回未找到
		return GeoFeedRow{}, false
	}

	row, find := FindGeoFeedRow(ip, rows)
	return row, find

}

func ReadGeoFeed() ([]GeoFeedRow, error) {
	path := viper.GetString("geoFeedPath")
	if path == "" {
		return nil, fmt.Errorf("geoFeedPath not configured")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	// 将 CSV 中的每一行转换为 GeoFeedRow 类型，并保存到 rowsSlice 中
	var rowsSlice []GeoFeedRow
	for _, row := range rows {
		cidr := row[0] // 假设第一列是 CIDR 字段
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			// 如果解析 CIDR 失败，跳过这一行
			continue
		}
		if len(row) == 4 {
			rowsSlice = append(rowsSlice, GeoFeedRow{
				IPNet:   ipnet,
				CIDR:    cidr,
				LtdCode: row[1],
				ISO3166: row[2],
				City:    row[3],
			})
		} else if len(row) >= 6 {
			rowsSlice = append(rowsSlice, GeoFeedRow{
				IPNet:   ipnet,
				CIDR:    cidr,
				LtdCode: row[1],
				ISO3166: row[2],
				City:    row[3],
				ASN:     row[4],
				IPWhois: row[5],
			})
		}

	}
	// 根据 CIDR 范围从小到大排序，方便后面查找
	sort.Slice(rowsSlice, func(i, j int) bool {
		return rowsSlice[i].IPNet.Mask.String() > rowsSlice[j].IPNet.Mask.String()
	})

	return rowsSlice, nil
}

func FindGeoFeedRow(ipStr string, rows []GeoFeedRow) (GeoFeedRow, bool) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		// 如果传入的 IP 无效，直接返回
		return GeoFeedRow{}, false
	}

	// 遍历每个 CIDR 范围，找到第一个包含传入的 IP 的 CIDR
	for _, row := range rows {
		if row.IPNet.Contains(ip) {
			return row, true
		}
	}

	return GeoFeedRow{}, false
}
