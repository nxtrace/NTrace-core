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
func MTRTablePrinter(stats []trace.MTRHopStat, iteration int, mode int, nameMode int, lang string, showIPs bool) {
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

		host := formatMTRHostWithMPLS(s, mode, nameMode, lang, showIPs)
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

	tbl.Print()
}

// MTRRenderTable 仅返回格式化后的行数据（用于测试/非终端场景）。
// mode / nameMode / lang 控制 Host 列内容；传 -1 / -1 / "" 等效于 HostModeFull + HostNamePTRorIP + "en"（向后兼容）。
func MTRRenderTable(stats []trace.MTRHopStat, mode int, nameMode int, lang string, showIPs bool) []MTRRow {
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
			Host:  formatMTRHostWithMPLS(s, mode, nameMode, lang, showIPs),
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
	HostModeBase  = 0 // 仅 IP/PTR
	HostModeASN   = 1 // ASN + IP/PTR
	HostModeCity  = 2 // ASN + IP/PTR + 城市
	HostModeOwner = 3 // ASN + IP/PTR + owner
	HostModeFull  = 4 // ASN + IP/PTR + full
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
//	nameMode == HostNamePTRorIP（默认）:
//	  - showIPs=false: 有 PTR 显示 PTR，否则 IP
//	  - showIPs=true:  有 PTR 且有 IP 时显示 "PTR (IP)"
//	都无   → "???"
func formatMTRHostBase(s trace.MTRHopStat, nameMode int, showIPs bool) string {
	if nameMode == HostNameIPOnly {
		if s.IP != "" {
			return s.IP
		}
		return "???"
	}

	if showIPs {
		if s.Host != "" && s.IP != "" {
			if s.Host == s.IP {
				return s.Host
			}
			return fmt.Sprintf("%s (%s)", s.Host, s.IP)
		}
		if s.Host != "" {
			return s.Host
		}
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
//	HostModeBase  (0): 仅 IP/PTR
//	HostModeASN   (1): ASN + IP/PTR
//	HostModeCity  (2): ASN + IP/PTR + 城市
//	HostModeOwner (3): ASN + IP/PTR + owner
//	HostModeFull  (4): ASN + IP/PTR + full
//
// ASN 始终作为前缀（对齐 mtr -rw 风格）：
//
//	"AS13335 one.one.one.one"    （模式 1）
//	"AS13335 one.one.one.one US" （模式 2）
func formatMTRHostByMode(s trace.MTRHopStat, mode int, nameMode int, lang string, showIPs bool) string {
	return joinMTRHostParts(buildMTRHostParts(s, mode, nameMode, lang, showIPs), ", ")
}

// formatMTRHostWithMPLS 构建 Host 列完整内容（含内联 MPLS），供表格打印器使用。
func formatMTRHostWithMPLS(s trace.MTRHopStat, mode int, nameMode int, lang string, showIPs bool) string {
	if mode < 0 {
		mode = HostModeFull
	}
	if nameMode < 0 {
		nameMode = HostNamePTRorIP
	}
	if lang == "" {
		lang = "en"
	}
	host := formatMTRHostByMode(s, mode, nameMode, lang, showIPs)
	if len(s.MPLS) > 0 {
		host += " " + strings.Join(s.MPLS, " ")
	}
	return host
}

// formatMTRHost 向后兼容别名（HostModeFull + HostNamePTRorIP + "en" + 内联 MPLS）。
func formatMTRHost(s trace.MTRHopStat) string {
	return formatMTRHostWithMPLS(s, HostModeFull, HostNamePTRorIP, "en", false)
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
func buildMTRHostParts(s trace.MTRHopStat, mode int, nameMode int, lang string, showIPs bool) mtrHostParts {
	if isWaitingHopStat(s) {
		return mtrHostParts{waiting: true}
	}

	parts := mtrHostParts{base: formatMTRHostBase(s, nameMode, showIPs)}
	if mode == HostModeBase || s.Geo == nil {
		return parts
	}
	parts.asn = mtrASNLabel(s.Geo)
	parts.extras = mtrGeoExtras(s.Geo, mode, lang)
	return parts
}

// buildTUIHostParts 构建仅供 TUI 使用的 host 组成部分。
//
// 与共享 buildMTRHostParts 不同，TUI 在 mode >= HostModeASN 时
// 对"有地址但缺失 ASN"的 hop 显示 AS???；HostModeBase 不显示 ASN。
func buildTUIHostParts(s trace.MTRHopStat, mode int, nameMode int, lang string, showIPs bool) mtrHostParts {
	p := buildMTRHostParts(s, mode, nameMode, lang, showIPs)
	if p.waiting {
		return p
	}
	// HostModeBase 不显示 ASN，也不显示 AS???
	if mode == HostModeBase {
		p.asn = ""
		return p
	}
	if p.asn == "" {
		p.asn = "AS???"
	}
	return p
}

// formatTUIHost 根据预先构建的 TUI host 组成和 ASN 列宽，生成手动空格对齐的 host 文本。
func formatTUIHost(parts mtrHostParts, asnW int) string {
	if parts.waiting {
		return "(waiting for reply)"
	}

	var b strings.Builder
	if asnW > 0 && parts.asn != "" {
		b.WriteString(padRight(parts.asn, asnW))
		b.WriteByte(' ')
	}
	b.WriteString(parts.base)
	if len(parts.extras) > 0 {
		b.WriteByte(' ')
		b.WriteString(strings.Join(parts.extras, " "))
	}
	return b.String()
}

// formatReportHost 构建 report 格式的 host 文本（空格分隔，waiting 感知，含 MPLS）。
func formatReportHost(s trace.MTRHopStat, mode int, nameMode int, lang string, showIPs bool) string {
	host := joinMTRHostParts(buildMTRHostParts(s, mode, nameMode, lang, showIPs), " ")
	if len(s.MPLS) > 0 {
		host += " " + strings.Join(s.MPLS, " ")
	}
	return host
}

// formatCompactReportHost 构建非 wide report 的精简 Host 文本。
//
// 规则：
//   - waiting → "(waiting for reply)"
//   - 仅显示 PTR/IP 基础信息
//   - 不显示 ASN / GEO / Owner / MPLS
func formatCompactReportHost(s trace.MTRHopStat, nameMode int, showIPs bool) string {
	if isWaitingHopStat(s) {
		return "(waiting for reply)"
	}
	return formatMTRHostBase(s, nameMode, showIPs)
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
	ShowIPs   bool
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
// 非 wide 模式仅显示 PTR/IP，不查询/展示 GEO，也不显示 MPLS，按终端宽度截断：
//
//	width < 100  → maxHost = 16
//	100 ≤ width < 140 → maxHost = 20
//	width ≥ 140  → maxHost = 24
func MTRReportPrint(stats []trace.MTRHopStat, opts MTRReportOptions) {
	lang := normalizeMTRReportLang(opts.Lang)
	fmt.Printf("Start: %s\n", opts.StartTime.Format("2006-01-02T15:04:05-0700"))

	hosts, hostColW := prepareMTRReportHosts(stats, opts, lang)
	printMTRReportHeader(opts, hostColW)
	printMTRReportRows(stats, hosts, hostColW)
}

func joinMTRHostParts(parts mtrHostParts, extrasSep string) string {
	if parts.waiting {
		return "(waiting for reply)"
	}
	segments := make([]string, 0, 3)
	if parts.asn != "" {
		segments = append(segments, parts.asn)
	}
	segments = append(segments, parts.base)
	if len(parts.extras) > 0 {
		segments = append(segments, strings.Join(parts.extras, extrasSep))
	}
	return strings.Join(segments, " ")
}

func mtrASNLabel(data *ipgeo.IPGeoData) string {
	if data == nil || data.Asnumber == "" {
		return ""
	}
	return "AS" + data.Asnumber
}

func mtrGeoExtras(data *ipgeo.IPGeoData, mode int, lang string) []string {
	switch mode {
	case HostModeBase, HostModeASN:
		return nil
	case HostModeCity:
		return singleMTRGeoExtra(mtrBestLocation(data, lang))
	case HostModeOwner:
		return singleMTRGeoExtra(mtrGeoOwner(data))
	default:
		return buildMTRFullGeoExtras(data, lang)
	}
}

func singleMTRGeoExtra(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

func mtrBestLocation(data *ipgeo.IPGeoData, lang string) string {
	if city := geoField(data.City, data.CityEn, lang); city != "" {
		return city
	}
	if prov := geoField(data.Prov, data.ProvEn, lang); prov != "" {
		return prov
	}
	return geoField(data.Country, data.CountryEn, lang)
}

func mtrGeoOwner(data *ipgeo.IPGeoData) string {
	if data == nil {
		return ""
	}
	if data.Owner != "" {
		return data.Owner
	}
	return data.Isp
}

func buildMTRFullGeoExtras(data *ipgeo.IPGeoData, lang string) []string {
	country := geoField(data.Country, data.CountryEn, lang)
	prov := geoField(data.Prov, data.ProvEn, lang)
	city := geoField(data.City, data.CityEn, lang)
	extras := make([]string, 0, 4)
	if country != "" {
		extras = append(extras, country)
	}
	if prov != "" && prov != country {
		extras = append(extras, prov)
	}
	if city != "" && city != prov {
		extras = append(extras, city)
	}
	if owner := mtrGeoOwner(data); owner != "" {
		extras = append(extras, owner)
	}
	return extras
}

func normalizeMTRReportLang(lang string) string {
	if lang == "" {
		return "cn"
	}
	return lang
}

func prepareMTRReportHosts(stats []trace.MTRHopStat, opts MTRReportOptions, lang string) ([]string, int) {
	hosts := make([]string, len(stats))
	for i, s := range stats {
		hosts[i] = buildMTRReportHost(s, opts, lang)
	}
	if opts.Wide {
		return hosts, computeWideMTRReportHostWidth(hosts, opts.SrcHost)
	}
	hostColW := narrowMTRReportHostWidth()
	truncateMTRReportHosts(hosts, hostColW)
	return hosts, hostColW
}

func buildMTRReportHost(s trace.MTRHopStat, opts MTRReportOptions, lang string) string {
	if opts.Wide {
		return formatReportHost(s, HostModeFull, HostNamePTRorIP, lang, opts.ShowIPs)
	}
	return formatCompactReportHost(s, HostNamePTRorIP, opts.ShowIPs)
}

func computeWideMTRReportHostWidth(hosts []string, srcHost string) int {
	hostColW := reportDisplayWidth(srcHost)
	for _, h := range hosts {
		if w := reportDisplayWidth(h); w > hostColW {
			hostColW = w
		}
	}
	return hostColW + 1
}

func narrowMTRReportHostWidth() int {
	tw, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || tw <= 0 {
		tw = 80
	}
	switch {
	case tw < 100:
		return 16
	case tw < 140:
		return 20
	default:
		return 24
	}
}

func truncateMTRReportHosts(hosts []string, hostColW int) {
	for i, h := range hosts {
		if reportDisplayWidth(h) > hostColW {
			hosts[i] = reportTruncateToWidth(h, hostColW)
		}
	}
}

func printMTRReportHeader(opts MTRReportOptions, hostColW int) {
	hostHeader := opts.SrcHost
	if !opts.Wide && reportDisplayWidth(hostHeader) > hostColW {
		hostHeader = reportTruncateToWidth(hostHeader, hostColW)
	}
	fmt.Printf("HOST: %s%s\n", reportPadRight(hostHeader, hostColW), mtrReportHeaderMetrics())
}

func mtrReportHeaderMetrics() string {
	const metricsFmt = " %6s %5s %6s %6s %6s %6s %6s"
	return fmt.Sprintf(metricsFmt, "Loss%", "Snt", "Last", "Avg", "Best", "Wrst", "StDev")
}

func printMTRReportRows(stats []trace.MTRHopStat, hosts []string, hostColW int) {
	prevTTL := 0
	for i, s := range stats {
		fmt.Printf("%s%s%s\n", mtrReportPrefix(s.TTL, prevTTL), reportPadRight(hosts[i], hostColW), formatMTRReportMetrics(s))
		prevTTL = s.TTL
	}
}

func mtrReportPrefix(ttl int, prevTTL int) string {
	if ttl == prevTTL {
		return "     "
	}
	return fmt.Sprintf("%3d. ", ttl)
}

func formatMTRReportMetrics(s trace.MTRHopStat) string {
	const metricsFmt = " %6s %5s %6s %6s %6s %6s %6s"
	m := formatMTRMetricStrings(s)
	return fmt.Sprintf(metricsFmt, m.loss, m.snt, m.last, m.avg, m.best, m.wrst, m.stdev)
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
