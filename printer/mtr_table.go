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
func MTRTablePrinter(stats []trace.MTRHopStat, iteration int) {
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

		host := formatMTRHost(s)
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
func MTRRenderTable(stats []trace.MTRHopStat) []MTRRow {
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
			Host:  formatMTRHost(s),
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

// formatMTRHost 构建 Host 列内容：IP/hostname + 简要 Geo + MPLS 标签。
func formatMTRHost(s trace.MTRHopStat) string {
	if s.IP == "" && s.Host == "" {
		return "???"
	}

	var parts []string

	// 主标识：hostname (IP) 或仅 IP
	if s.Host != "" && s.IP != "" {
		parts = append(parts, fmt.Sprintf("%s (%s)", s.Host, s.IP))
	} else if s.Host != "" {
		parts = append(parts, s.Host)
	} else {
		parts = append(parts, s.IP)
	}

	// 简要 Geo 信息
	if s.Geo != nil {
		geo := formatMTRGeoData(s.Geo)
		if geo != "" {
			parts = append(parts, geo)
		}
	}

	// MPLS 标签（extractMPLS 已产出 [MPLS: Lbl ...] 格式，直接拼接）
	if len(s.MPLS) > 0 {
		parts = append(parts, strings.Join(s.MPLS, " "))
	}

	return strings.Join(parts, " ")
}

// formatMTRGeoData 返回简短 geo 描述，例如 "AS13335, US" 或 "AS4134, China, Beijing"。
func formatMTRGeoData(data *ipgeo.IPGeoData) string {
	if data == nil {
		return ""
	}
	var segs []string

	if data.Asnumber != "" {
		segs = append(segs, "AS"+data.Asnumber)
	}

	// 使用英文字段
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
