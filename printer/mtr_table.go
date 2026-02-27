package printer

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/rodaine/table"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
)

// ---------------------------------------------------------------------------
// MTR 表格打印器
// ---------------------------------------------------------------------------

// MTRTablePrinter 将 MTR 快照渲染为经典 MTR 风格表格。
// 每次调用都会先清屏再重绘。
func MTRTablePrinter(stats []trace.MTRHopStat, iteration int, mode int, nameMode int, lang string) {
	// 清屏并移动到左上角
	fmt.Print("\033[H\033[2J")

	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()

	tbl := table.New("Hop", "Loss%", "Snt", "Last", "Avg", "Best", "Wrst", "StDev", "Host")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	prevTTL := 0
	for _, s := range stats {
		hopStr := fmt.Sprint(s.TTL)
		if s.TTL == prevTTL {
			hopStr = "" // 同一 TTL 的多路径不重复显示跳数
		}
		prevTTL = s.TTL

		host := formatMTRHostWithMPLS(s, mode, nameMode, lang)
		tbl.AddRow(
			hopStr,
			formatLoss(s.Loss),
			s.Snt,
			formatMs(s.Last),
			formatMs(s.Avg),
			formatMs(s.Best),
			formatMs(s.Wrst),
			formatMs(s.StDev),
			host,
		)
	}

	fmt.Fprintf(color.Output, "Round: %d\n", iteration)
	tbl.Print()
}

// MTRRenderTable 仅返回格式化后的行数据（用于测试/非终端场景）。
// mode / nameMode / lang 控制 Host 列内容；传 -1 / -1 / "" 等效于 HostModeFull + HostNamePTRorIP + "en"（向后兼容）。
func MTRRenderTable(stats []trace.MTRHopStat, mode int, nameMode int, lang string) []MTRRow {
	prevTTL := 0
	rows := make([]MTRRow, 0, len(stats))
	for _, s := range stats {
		hopStr := fmt.Sprint(s.TTL)
		if s.TTL == prevTTL {
			hopStr = ""
		}
		prevTTL = s.TTL

		rows = append(rows, MTRRow{
			Hop:   hopStr,
			Loss:  formatLoss(s.Loss),
			Snt:   fmt.Sprint(s.Snt),
			Last:  formatMs(s.Last),
			Avg:   formatMs(s.Avg),
			Best:  formatMs(s.Best),
			Wrst:  formatMs(s.Wrst),
			StDev: formatMs(s.StDev),
			Host:  formatMTRHostWithMPLS(s, mode, nameMode, lang),
		})
	}
	return rows
}

// MTRRow 表示表格中一行经过格式化的数据。
type MTRRow struct {
	Hop   string
	Loss  string
	Snt   string
	Last  string
	Avg   string
	Best  string
	Wrst  string
	StDev string
	Host  string
}

// ---------------------------------------------------------------------------
// 格式化辅助
// ---------------------------------------------------------------------------

// formatLoss 返回 "0.0%"、"100.0%" 等。
func formatLoss(pct float64) string {
	return fmt.Sprintf("%.1f%%", pct)
}

// formatMs 返回 "12.34" —— 毫秒值保留两位小数。
func formatMs(ms float64) string {
	return fmt.Sprintf("%.2f", ms)
}

// ---------------------------------------------------------------------------
// 显示模式常量
// ---------------------------------------------------------------------------

const (
	HostModeASN   = 0 // ASN + PTR/IP（默认）
	HostModeCity  = 1 // + 城市/省份/国家
	HostModeOwner = 2 // + 运营商
	HostModeFull  = 3 // + 完整地址 + 运营商
)

// ---------------------------------------------------------------------------
// Host 基础显示模式（n 键切换）
// ---------------------------------------------------------------------------

const (
	HostNamePTRorIP = 0 // 默认：有 PTR 显示 PTR，否则 IP
	HostNameIPOnly  = 1 // 始终显示 IP
)

// ---------------------------------------------------------------------------
// Host 列格式化（多模式 + 语言感知）
// ---------------------------------------------------------------------------

// formatMTRHostBase 构建基础 host 标识。
//
//	nameMode == HostNameIPOnly → 始终显示 IP
//	nameMode == HostNamePTRorIP（默认）→ 有 PTR 显示 PTR，否则 IP
//	都无   → "???"
func formatMTRHostBase(s trace.MTRHopStat, nameMode int) string {
	if nameMode == HostNameIPOnly {
		if s.IP != "" {
			return s.IP
		}
		return "???"
	}
	// HostNamePTRorIP（默认）
	if s.Host != "" {
		return s.Host
	}
	if s.IP != "" {
		return s.IP
	}
	return "???"
}

// geoField 根据语言选择中/英字段。
// lang == "en" 优先英文，否则优先中文。
func geoField(cn, en, lang string) string {
	if lang == "en" {
		if en != "" {
			return en
		}
		return cn
	}
	// 默认（含 "cn"）优先中文
	if cn != "" {
		return cn
	}
	return en
}

// formatMTRHostByMode 按显示模式构建 Host 列（不含 MPLS）。
//
//	HostModeASN   (0): ASN + PTR/IP
//	HostModeCity  (1): + 城市/省份/国家
//	HostModeOwner (2): + 运营商
//	HostModeFull  (3): + 完整地址 + 运营商
func formatMTRHostByMode(s trace.MTRHopStat, mode int, nameMode int, lang string) string {
	base := formatMTRHostBase(s, nameMode)
	if s.Geo == nil {
		return base
	}

	var segs []string

	// ASN 在所有模式下都展示
	if s.Geo.Asnumber != "" {
		segs = append(segs, "AS"+s.Geo.Asnumber)
	}

	switch mode {
	case HostModeASN:
		// 仅 ASN，无额外 geo
	case HostModeCity:
		// 单级回退链：city -> prov -> country，取最具体的一个
		city := geoField(s.Geo.City, s.Geo.CityEn, lang)
		prov := geoField(s.Geo.Prov, s.Geo.ProvEn, lang)
		country := geoField(s.Geo.Country, s.Geo.CountryEn, lang)
		if city != "" {
			segs = append(segs, city)
		} else if prov != "" {
			segs = append(segs, prov)
		} else if country != "" {
			segs = append(segs, country)
		}
	case HostModeOwner:
		owner := s.Geo.Owner
		if owner == "" {
			owner = s.Geo.Isp
		}
		if owner != "" {
			segs = append(segs, owner)
		}
	default: // HostModeFull
		country := geoField(s.Geo.Country, s.Geo.CountryEn, lang)
		prov := geoField(s.Geo.Prov, s.Geo.ProvEn, lang)
		city := geoField(s.Geo.City, s.Geo.CityEn, lang)
		if country != "" {
			segs = append(segs, country)
		}
		if prov != "" && prov != country {
			segs = append(segs, prov)
		}
		if city != "" && city != prov {
			segs = append(segs, city)
		}
		owner := s.Geo.Owner
		if owner == "" {
			owner = s.Geo.Isp
		}
		if owner != "" {
			segs = append(segs, owner)
		}
	}

	geo := strings.Join(segs, ", ")
	if geo == "" {
		return base
	}
	return base + " " + geo
}

// formatMTRHostWithMPLS 构建 Host 列完整内容（含内联 MPLS），供表格打印器使用。
func formatMTRHostWithMPLS(s trace.MTRHopStat, mode int, nameMode int, lang string) string {
	if mode < 0 {
		mode = HostModeFull
	}
	if nameMode < 0 {
		nameMode = HostNamePTRorIP
	}
	if lang == "" {
		lang = "en"
	}
	host := formatMTRHostByMode(s, mode, nameMode, lang)
	if len(s.MPLS) > 0 {
		host += " " + strings.Join(s.MPLS, " ")
	}
	return host
}

// formatMTRHost 向后兼容别名（HostModeFull + HostNamePTRorIP + "en" + 内联 MPLS）。
func formatMTRHost(s trace.MTRHopStat) string {
	return formatMTRHostWithMPLS(s, HostModeFull, HostNamePTRorIP, "en")
}

// formatMTRGeoData 返回简短 geo 描述（向后兼容，等效于英文 HostModeFull geo 部分）。
func formatMTRGeoData(data *ipgeo.IPGeoData) string {
	if data == nil {
		return ""
	}
	var segs []string

	if data.Asnumber != "" {
		segs = append(segs, "AS"+data.Asnumber)
	}

	country := data.CountryEn
	if country == "" {
		country = data.Country
	}
	prov := data.ProvEn
	if prov == "" {
		prov = data.Prov
	}
	city := data.CityEn
	if city == "" {
		city = data.City
	}

	if country != "" {
		segs = append(segs, country)
	}
	if prov != "" && prov != country {
		segs = append(segs, prov)
	}
	if city != "" && city != prov {
		segs = append(segs, city)
	}

	owner := data.Owner
	if owner == "" {
		owner = data.Isp
	}
	if owner != "" {
		segs = append(segs, owner)
	}

	return strings.Join(segs, ", ")
}
