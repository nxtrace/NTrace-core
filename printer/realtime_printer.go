package printer

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/xgadget-lab/nexttrace/ipgeo"
	"github.com/xgadget-lab/nexttrace/trace"
)

type HopInfo int

const (
	General HopInfo = 0
	IXP     HopInfo = 1
	Peer    HopInfo = 2
	PoP     HopInfo = 3
	Aboard  HopInfo = 4
)

func findLatestAvailableHop(res *trace.Result, ttl int, probesIndex int) int {
	for ttl > 0 {
		// 查找上一个跃点是不是有效结果
		ttl--
		if res.Hops[ttl][probesIndex].Address != nil {
			return ttl
		}
	}
	// 没找到
	return -1
}

func unifyName(name string) string {
	if name == "China" || name == "CN" {
		return "中国"
	} else if name == "Hong kong" || name == "香港" || name == "Central and Western" {
		return "中国香港"
	} else if name == "Taiwan" || name == "台湾" {
		return "中国台湾"
	} else {
		return name
	}
}

func chinaISPPeer(hostname string) bool {
	var keyWords = []string{"china", "ct", "cu", "cm", "cnc", "4134", "4837", "4809", "9929"}
	for _, k := range keyWords {
		if strings.Contains(strings.ToLower(hostname), k) {
			return true
		}
	}
	return false
}

func chinaMainland(h trace.Hop) bool {
	if unifyName(h.Geo.Country) == "中国" && unifyName(h.Geo.Prov) != "中国香港" && unifyName(h.Geo.Prov) != "中国台湾" {
		return true
	} else {
		return false
	}
}

func makeHopsType(res *trace.Result, ttl int) map[int]HopInfo {
	// 创建一个字典，存放所有当前TTL的跃点类型集合
	hopProbesMap := make(map[int]HopInfo)
	for i := range res.Hops[ttl] {
		// 判断是否Hops以及Geo结构体已经初始化
		if res.Hops[ttl][i].Address != nil && reflect.DeepEqual(res.Hops[ttl][i].Geo, ipgeo.IPGeoData{}) {
			if availableTTL := findLatestAvailableHop(res, ttl, i); availableTTL != -1 {
				switch {
				case strings.Contains(res.Hops[ttl][i].Geo.District, "IXP") || strings.Contains(strings.ToLower(res.Hops[ttl][i].Hostname), "ix"):
					hopProbesMap[i] = IXP
				case strings.Contains(res.Hops[ttl][i].Geo.District, "Peer") || chinaISPPeer(res.Hops[ttl][i].Hostname):
					hopProbesMap[i] = Peer
				case strings.Contains(res.Hops[ttl][i].Geo.District, "PoP"):
					hopProbesMap[i] = PoP
				// 2个有效跃点必须都为有效数据
				case res.Hops[availableTTL][i].Geo.Country != "LAN Address" && res.Hops[ttl][i].Geo.Country != "LAN Address" &&
					res.Hops[availableTTL][i].Geo.Country != "" && res.Hops[ttl][i].Geo.Country != "" &&
					chinaMainland(res.Hops[availableTTL][i]) != chinaMainland(res.Hops[ttl][i]):
					hopProbesMap[i] = Aboard
				}
			} else {
				hopProbesMap[i] = General
			}
		}
	}

	return hopProbesMap
}

func RealtimePrinter(res *trace.Result, ttl int) {
	fmt.Print(ttl + 1)
	hopsTypeMap := makeHopsType(res, ttl)
	for i := range res.Hops[ttl] {
		HopPrinter(res.Hops[ttl][i], hopsTypeMap[i])
	}
}
