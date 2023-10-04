package printer

import (
	"fmt"
	"strings"

	"github.com/nxtrace/NTrace-core/trace"
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
		// 判断此TTL跃点是否有效并判断地理位置结构体是否已经初始化
		if len(res.Hops[ttl]) != 0 && res.Hops[ttl][probesIndex].Success && res.Hops[ttl][probesIndex].Geo != nil {
			// TTL虽有效，但地理位置API没有能够正确返回数据，依旧不能视为有效数据
			if res.Hops[ttl][probesIndex].Geo.Country == "" {
				// 跳过继续寻找上一个有效跃点
				continue
			}
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
		// 判断是否res.Hops[ttl][i]是一个有效的跃点并且地理位置信息已经初始化
		if res.Hops[ttl][i].Success && res.Hops[ttl][i].Geo != nil {
			if availableTTL := findLatestAvailableHop(res, ttl, i); availableTTL != -1 {
				switch {
				case strings.Contains(res.Hops[ttl][i].Geo.District, "IXP") || strings.Contains(strings.ToLower(res.Hops[ttl][i].Hostname), "ix"):
					hopProbesMap[i] = IXP
				case strings.Contains(res.Hops[ttl][i].Geo.District, "Peer") || chinaISPPeer(res.Hops[ttl][i].Hostname):
					hopProbesMap[i] = Peer
				case strings.Contains(res.Hops[ttl][i].Geo.District, "PoP"):
					hopProbesMap[i] = PoP
				// 2个有效跃点必须都为有效数据，如果当前跳没有地理位置信息或者为局域网，不能视为有效节点
				case res.Hops[availableTTL][i].Geo.Country != "LAN Address" && res.Hops[ttl][i].Geo.Country != "LAN Address" && res.Hops[ttl][i].Geo.Country != "" &&
					// 一个跃点在中国大陆，另外一个跃点在其他地区，则可以推断出数据包跨境
					chinaMainland(res.Hops[availableTTL][i]) != chinaMainland(res.Hops[ttl][i]):
					// TODO: 将先后2跳跃点信息汇报给API，以完善相关数据
					hopProbesMap[i] = Aboard
				}
			} else {
				hopProbesMap[i] = General
			}
		}
	}

	return hopProbesMap
}

func ClassicPrinter(res *trace.Result, ttl int) {
	fmt.Print(ttl + 1)
	hopsTypeMap := makeHopsType(res, ttl)
	for i := range res.Hops[ttl] {
		HopPrinter(res.Hops[ttl][i], hopsTypeMap[i])
	}
}
