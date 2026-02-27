package printer

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/mattn/go-runewidth"
	"github.com/rodaine/table"
	"golang.org/x/term"

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
		m := formatMTRMetricStrings(s)
		tbl.AddRow(
			hopStr,
			m.loss,
			m.snt,
			m.last,
			m.avg,
			m.best,
			m.wrst,
			m.stdev,
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

		m := formatMTRMetricStrings(s)
		rows = append(rows, MTRRow{
			Hop:   hopStr,
			Loss:  m.loss,
			Snt:   m.snt,
			Last:  m.last,
			Avg:   m.avg,
			Best:  m.best,
			Wrst:  m.wrst,
			StDev: m.stdev,
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

// isWaitingHopStat 判断该 hop 是否为 "(waiting for reply)" 状态。
// 条件：100% 丢包（≥ 99.95% 避免浮点边界抖动）且无 IP/Host。
func isWaitingHopStat(s trace.MTRHopStat) bool {
	return s.Loss >= 99.95 && s.IP == "" && s.Host == ""
}

// mtrMetrics 存储已格式化的指标字符串。
type mtrMetrics struct {
	loss, snt, last, avg, best, wrst, stdev string
}

// formatMTRMetricStrings 返回已格式化的指标字符串。
// waiting 行全部返回空字符串。
func formatMTRMetricStrings(s trace.MTRHopStat) mtrMetrics {
	if isWaitingHopStat(s) {
		return mtrMetrics{}
	}
	return mtrMetrics{
		loss:  formatLoss(s.Loss),
		snt:   fmt.Sprint(s.Snt),
		last:  formatMs(s.Last),
		avg:   formatMs(s.Avg),
		best:  formatMs(s.Best),
		wrst:  formatMs(s.Wrst),
		stdev: formatMs(s.StDev),
	}
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
//
// ASN 始终作为前缀（对齐 mtr -rw 风格）：
//
//	"AS13335 one.one.one.one"    （模式 0）
//	"AS13335 one.one.one.one US" （模式 1）
func formatMTRHostByMode(s trace.MTRHopStat, mode int, nameMode int, lang string) string {
	base := formatMTRHostBase(s, nameMode)
	if s.Geo == nil {
		return base
	}

	// ASN 作为前缀
	asnPrefix := ""
	if s.Geo.Asnumber != "" {
		asnPrefix = "AS" + s.Geo.Asnumber
	}

	var segs []string

	switch mode {
	case HostModeASN:
		// 仅 ASN，无额外 geo
	case HostModeCity:
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

	// 拼接顺序：asnPrefix + base + geo
	var parts []string
	if asnPrefix != "" {
		parts = append(parts, asnPrefix)
	}
	parts = append(parts, base)
	geo := strings.Join(segs, ", ")
	if geo != "" {
		parts = append(parts, geo)
	}
	return strings.Join(parts, " ")
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

// ---------------------------------------------------------------------------
// 结构化 Host 组成（TUI / Report 共用）
// ---------------------------------------------------------------------------

// mtrHostParts 包含 Host 行的各组成部分，便于不同输出层（TUI/report）组装。
type mtrHostParts struct {
	waiting bool     // loss ≥ 99.95% 且无地址 → 显示 (waiting for reply)
	asn     string   // "AS13335" 或 ""
	base    string   // IP 或 PTR
	extras  []string // geo/owner 等附加字段（不含 ASN）
}

// buildMTRHostParts 从统计数据构建 host 各组成部分。
//
// waiting 条件：loss ≥ 99.95%（避免浮点边界抖动）且无 Host 和 IP。
// 若 loss=100% 但仍有 IP/Host（极少见），优先显示真实地址。
func buildMTRHostParts(s trace.MTRHopStat, mode int, nameMode int, lang string) mtrHostParts {
	if isWaitingHopStat(s) {
		return mtrHostParts{waiting: true}
	}

	base := formatMTRHostBase(s, nameMode)

	var asn string
	var extras []string

	if s.Geo != nil {
		if s.Geo.Asnumber != "" {
			asn = "AS" + s.Geo.Asnumber
		}

		switch mode {
		case HostModeASN:
			// 仅 ASN，无额外 geo
		case HostModeCity:
			city := geoField(s.Geo.City, s.Geo.CityEn, lang)
			prov := geoField(s.Geo.Prov, s.Geo.ProvEn, lang)
			country := geoField(s.Geo.Country, s.Geo.CountryEn, lang)
			if city != "" {
				extras = append(extras, city)
			} else if prov != "" {
				extras = append(extras, prov)
			} else if country != "" {
				extras = append(extras, country)
			}
		case HostModeOwner:
			owner := s.Geo.Owner
			if owner == "" {
				owner = s.Geo.Isp
			}
			if owner != "" {
				extras = append(extras, owner)
			}
		default: // HostModeFull
			country := geoField(s.Geo.Country, s.Geo.CountryEn, lang)
			prov := geoField(s.Geo.Prov, s.Geo.ProvEn, lang)
			city := geoField(s.Geo.City, s.Geo.CityEn, lang)
			if country != "" {
				extras = append(extras, country)
			}
			if prov != "" && prov != country {
				extras = append(extras, prov)
			}
			if city != "" && city != prov {
				extras = append(extras, city)
			}
			owner := s.Geo.Owner
			if owner == "" {
				owner = s.Geo.Isp
			}
			if owner != "" {
				extras = append(extras, owner)
			}
		}
	}

	return mtrHostParts{
		asn:    asn,
		base:   base,
		extras: extras,
	}
}

// formatTUIHost 构建 TUI 格式的 host 文本（tab 分隔，waiting 感知）。
//
// 规则：
//   - waiting → "(waiting for reply)"
//   - ASN\tIP/PTR\t后续信息（空格分隔）
//   - 无 ASN 时省略 ASN + 首个 tab
//   - 无后续信息时省略末尾 tab
func formatTUIHost(s trace.MTRHopStat, mode int, nameMode int, lang string) string {
	p := buildMTRHostParts(s, mode, nameMode, lang)
	if p.waiting {
		return "(waiting for reply)"
	}

	var b strings.Builder
	if p.asn != "" {
		b.WriteString(p.asn)
		b.WriteByte('\t')
	}
	b.WriteString(p.base)
	if len(p.extras) > 0 {
		b.WriteByte('\t')
		b.WriteString(strings.Join(p.extras, " "))
	}
	return b.String()
}

// formatReportHost 构建 report 格式的 host 文本（空格分隔，waiting 感知，含 MPLS）。
func formatReportHost(s trace.MTRHopStat, mode int, nameMode int, lang string) string {
	p := buildMTRHostParts(s, mode, nameMode, lang)
	if p.waiting {
		return "(waiting for reply)"
	}

	var parts []string
	if p.asn != "" {
		parts = append(parts, p.asn)
	}
	parts = append(parts, p.base)
	if len(p.extras) > 0 {
		parts = append(parts, strings.Join(p.extras, " "))
	}
	host := strings.Join(parts, " ")
	if len(s.MPLS) > 0 {
		host += " " + strings.Join(s.MPLS, " ")
	}
	return host
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

// ---------------------------------------------------------------------------
// MTR Report 模式打印器（对齐 mtr -rzw 风格）
// ---------------------------------------------------------------------------

// MTRReportOptions 控制报告输出细节。
type MTRReportOptions struct {
	StartTime time.Time
	SrcHost   string
	Wide      bool
	Lang      string
}

// MTRReportPrint 以 mtr -rzw 风格将最终统计一次性输出到 stdout。
//
// 输出格式（示例）：
//
//	Start: 2025-07-14T09:12:00+0800
//	HOST: myhost                       Loss%   Snt   Last    Avg   Best   Wrst  StDev
//	  1. AS4134 one.one.one.one         0.0%    10    1.23   1.45   0.98   2.10   0.32
//	  2. ???                           100.0%    10    0.00   0.00   0.00   0.00   0.00
//
// Wide 模式下使用 HostModeFull（完整地址 + 运营商），host 列宽度取所有行最大值；
// 非 wide 模式使用 HostModeASN，按终端宽度截断：
//
//	width < 100  → maxHost = 16
//	100 ≤ width < 140 → maxHost = 20
//	width ≥ 140  → maxHost = 24
func MTRReportPrint(stats []trace.MTRHopStat, opts MTRReportOptions) {
	lang := opts.Lang
	if lang == "" {
		lang = "cn"
	}

	// wide 模式使用完整地址，非 wide 仅 ASN
	hostMode := HostModeASN
	if opts.Wide {
		hostMode = HostModeFull
	}

	// Start 行
	fmt.Printf("Start: %s\n", opts.StartTime.Format("2006-01-02T15:04:05-0700"))

	// 预先格式化所有 host 字符串，以便确定对齐宽度
	hosts := make([]string, len(stats))
	for i, s := range stats {
		hosts[i] = formatReportHost(s, hostMode, HostNamePTRorIP, lang)
	}

	// 确定 host 列对齐宽度
	var hostColW int
	if opts.Wide {
		// wide: 取所有 host + 表头中的最大可视宽度
		hostColW = reportDisplayWidth(opts.SrcHost)
		for _, h := range hosts {
			if w := reportDisplayWidth(h); w > hostColW {
				hostColW = w
			}
		}
		hostColW++ // 加 1 列间距
	} else {
		tw, _, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil || tw <= 0 {
			tw = 80
		}
		switch {
		case tw < 100:
			hostColW = 16
		case tw < 140:
			hostColW = 20
		default:
			hostColW = 24
		}
		// 截断 host 到 hostColW
		for i, h := range hosts {
			if reportDisplayWidth(h) > hostColW {
				hosts[i] = reportTruncateToWidth(h, hostColW)
			}
		}
	}

	// 度量列格式：右对齐，固定宽度
	const metricsFmt = " %6s %5s %6s %6s %6s %6s %6s"

	// HOST 表头行
	hostHeader := opts.SrcHost
	if !opts.Wide && reportDisplayWidth(hostHeader) > hostColW {
		hostHeader = reportTruncateToWidth(hostHeader, hostColW)
	}

	headerMetrics := fmt.Sprintf(metricsFmt, "Loss%", "Snt", "Last", "Avg", "Best", "Wrst", "StDev")
	fmt.Printf("HOST: %s%s\n", reportPadRight(hostHeader, hostColW), headerMetrics)

	// 数据行
	// 前缀 "  N. " 占 5 字符（%3d. + 空格），与 "HOST: " 的 6 字符差 1，
	// 将额外 1 格留给 host 列左侧自然padding。
	prevTTL := 0
	for i, s := range stats {
		prefix := fmt.Sprintf("%3d. ", s.TTL)
		if s.TTL == prevTTL {
			prefix = "     "
		}
		prevTTL = s.TTL

		m := formatMTRMetricStrings(s)
		metrics := fmt.Sprintf(metricsFmt,
			m.loss,
			m.snt,
			m.last,
			m.avg,
			m.best,
			m.wrst,
			m.stdev,
		)
		fmt.Printf("%s%s%s\n", prefix, reportPadRight(hosts[i], hostColW), metrics)
	}
}

// reportDisplayWidth 返回字符串的终端显示宽度（CJK 字符占 2 列）。
func reportDisplayWidth(s string) int {
	return runewidth.StringWidth(s)
}

// reportTruncateToWidth 将字符串按终端显示宽度截断（CJK 安全）。
func reportTruncateToWidth(s string, maxW int) string {
	if runewidth.StringWidth(s) <= maxW {
		return s
	}
	return runewidth.Truncate(s, maxW, "")
}

// reportPadRight 将 s 用空格右填充到 width 显示列宽（CJK 安全）。
func reportPadRight(s string, width int) string {
	w := runewidth.StringWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
