package printer

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"

	"github.com/nxtrace/NTrace-core/trace"
)

// ---------------------------------------------------------------------------
// MTR TUI 全屏帧渲染器（mtr 风格自适应布局）
// ---------------------------------------------------------------------------

// MTRTUIStatus 表示 TUI 当前运行状态。
type MTRTUIStatus int

const (
	MTRTUIRunning MTRTUIStatus = iota
	MTRTUIPaused
)

// MTRTUIHeader 包含帧顶部显示的元信息。
type MTRTUIHeader struct {
	Target    string
	StartTime time.Time
	Status    MTRTUIStatus
	Iteration int
	// 以下为 v2 新增字段
	Domain   string // 用户输入的域名（可为空）
	TargetIP string // 解析后的目标 IP
	Version  string // 软件版本，如 "v1.3.0"
	// 以下为 v3 新增字段
	SrcHost     string // 源主机名
	SrcIP       string // 源 IP
	Lang        string // 语言（"en" / "cn"）
	DisplayMode int    // 显示模式 0-3
	NameMode    int    // Host 基础显示 0=PTR/IP, 1=IP only
	APIInfo     string // preferred API 信息（纯文本，可为空）
}

// ---------------------------------------------------------------------------
// 布局计算器
// ---------------------------------------------------------------------------

// mtrTUILayout 描述一帧布局参数，由终端宽度动态计算。
type mtrTUILayout struct {
	termWidth    int
	prefixW      int // hop prefix 列宽（如 "10.|--"）
	hostW        int // Host 列显示宽度
	lossW        int // Loss% 列宽
	sntW         int // Snt 列宽
	lastW        int // Last 列宽
	avgW         int // Avg 列宽
	bestW        int // Best 列宽
	wrstW        int // Wrst 列宽
	stdevW       int // StDev 列宽
	metricsStart int // 指标区起始列（0-based）
}

// metricsWidth 返回右侧指标区总显示宽度（7 列 + 6 个间距）。
func (lo *mtrTUILayout) metricsWidth() int {
	return lo.lossW + lo.sntW + lo.lastW + lo.avgW + lo.bestW + lo.wrstW + lo.stdevW + 6*tuiMetricGap
}

// totalWidth 返回一行数据的总显示宽度。
func (lo *mtrTUILayout) totalWidth() int {
	return lo.prefixW + tuiPrefixGap + lo.hostW + tuiHostGap + lo.metricsWidth()
}

// 各列默认与最小宽度
const (
	tuiPrefixW     = 6 // "10.|--" → max 6 显示宽度
	tuiPrefixGap   = 1 // prefix 与 Host 之间间距
	tuiHostGap     = 2 // Host 与指标区之间间距
	tuiMetricGap   = 1 // 指标列之间间距
	tuiDefaultTerm = 120

	tuiLossDefault = 6
	tuiSntDefault  = 4
	tuiRTTDefault  = 8
	tuiHostDefault = 40
	tuiHostMin     = 8
	tuiLossMin     = 5
	tuiSntMin      = 3
	tuiRTTMin      = 6
)

// computeLayout 根据终端宽度计算布局。
//
// 三阶段压缩策略：
//  1. 默认指标宽度，Host 取剩余空间
//  2. Host 降至 tuiHostMin，按比例压缩指标列
//  3. 极窄场景：循环缩减 Host（最低 1 列）直到 totalWidth ≤ termWidth
//
// 绝对下限 totalWidth = prefixW(6)+prefixGap(1)+host(1)+hostGap(2)+7×1+6×1 = 23。
// 当 termWidth < 23 时接受溢出——该宽度下终端本身已不可用。
func computeLayout(termWidth int) mtrTUILayout {
	if termWidth <= 0 {
		termWidth = tuiDefaultTerm
	}

	lo := mtrTUILayout{
		termWidth: termWidth,
		prefixW:   tuiPrefixW,
		lossW:     tuiLossDefault,
		sntW:      tuiSntDefault,
		lastW:     tuiRTTDefault,
		avgW:      tuiRTTDefault,
		bestW:     tuiRTTDefault,
		wrstW:     tuiRTTDefault,
		stdevW:    tuiRTTDefault,
	}

	// 左侧固定部分 = prefix + gap
	leftFixed := lo.prefixW + tuiPrefixGap

	// --- Phase 1: 默认指标，Host 取剩余 ---
	hostW := termWidth - leftFixed - tuiHostGap - lo.metricsWidth()
	if hostW >= tuiHostMin {
		lo.hostW = hostW
		lo.metricsStart = leftFixed + lo.hostW + tuiHostGap
		return lo
	}

	// --- Phase 2: Host 降至 tuiHostMin，压缩指标 ---
	lo.hostW = tuiHostMin
	metricsAvail := termWidth - leftFixed - lo.hostW - tuiHostGap
	lo.lossW, lo.sntW, lo.lastW, lo.avgW, lo.bestW, lo.wrstW, lo.stdevW =
		shrinkMetrics(metricsAvail)

	// --- Phase 3: 极窄——循环缩减 Host 直到不超宽（最低 1） ---
	for lo.totalWidth() > termWidth && lo.hostW > 1 {
		lo.hostW--
	}

	// --- 右锚定：把剩余 slack 全部回填 hostW，保证指标区贴右边界 ---
	if slack := termWidth - lo.totalWidth(); slack > 0 {
		lo.hostW += slack
	}

	lo.metricsStart = leftFixed + lo.hostW + tuiHostGap
	return lo
}

// shrinkMetrics 在 available 宽度内缩小 7 列指标 + 6 间距。
//
// 当 available 极小时，列宽可降至绝对下限 1，确保 computeLayout
// 的 phase-3 循环能把 totalWidth 压到 termWidth 以内。
func shrinkMetrics(available int) (lossW, sntW, lastW, avgW, bestW, wrstW, stdevW int) {
	avail := available - 6*tuiMetricGap
	if avail < 7 {
		// 绝对下限：每列 1
		return 1, 1, 1, 1, 1, 1, 1
	}

	defaults := [7]int{tuiLossDefault, tuiSntDefault, tuiRTTDefault, tuiRTTDefault, tuiRTTDefault, tuiRTTDefault, tuiRTTDefault}
	total := 0
	for _, c := range defaults {
		total += c
	}
	if avail >= total {
		return defaults[0], defaults[1], defaults[2], defaults[3], defaults[4], defaults[5], defaults[6]
	}

	// 常规最小值
	mins := [7]int{tuiLossMin, tuiSntMin, tuiRTTMin, tuiRTTMin, tuiRTTMin, tuiRTTMin, tuiRTTMin}
	minTotal := 0
	for _, m := range mins {
		minTotal += m
	}

	var cols [7]int
	if avail >= minTotal {
		// 按比例缩小，兜底到常规最小值
		for i := range cols {
			w := defaults[i] * avail / total
			if w < mins[i] {
				w = mins[i]
			}
			cols[i] = w
		}
	} else {
		// 极限缩小，兜底到 1
		for i := range cols {
			w := defaults[i] * avail / total
			if w < 1 {
				w = 1
			}
			cols[i] = w
		}
	}
	return cols[0], cols[1], cols[2], cols[3], cols[4], cols[5], cols[6]
}

// ---------------------------------------------------------------------------
// 显示宽度辅助（CJK 宽字符感知）
// ---------------------------------------------------------------------------

// displayWidth 返回字符串的终端显示宽度。
func displayWidth(s string) int {
	return runewidth.StringWidth(s)
}

// truncateByDisplayWidth 将 s 截断到不超过 max 个显示列。
// 超长时追加 "."。
func truncateByDisplayWidth(s string, max int) string {
	if max <= 0 {
		return ""
	}
	w := runewidth.StringWidth(s)
	if w <= max {
		return s
	}
	if max <= 1 {
		return "."
	}
	return runewidth.Truncate(s, max-1, "") + "."
}

// padRight 将 s 用空格填充到 width 显示列宽（CJK 安全）。
func padRight(s string, width int) string {
	w := runewidth.StringWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// padLeft 将 s 左填充空格到 width 显示列宽。
func padLeft(s string, width int) string {
	w := runewidth.StringWidth(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}

// fitRight 先截断到 width，再右对齐填充。
// 当列宽小于内容宽度时严格截断，保证输出恰好 width 列。
func fitRight(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = truncateByDisplayWidth(s, width)
	return padLeft(s, width)
}

// fitLeft 先截断到 width，再左对齐填充。
func fitLeft(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = truncateByDisplayWidth(s, width)
	return padRight(s, width)
}

// ---------------------------------------------------------------------------
// 帧渲染
// ---------------------------------------------------------------------------

// tuiLine 在 raw mode 下输出一行并以 \r\n 结束，
// 确保光标回到行首——裸 \n 在 raw mode 下只向下移动不回列。
func tuiLine(b *strings.Builder, format string, a ...any) {
	fmt.Fprintf(b, format, a...)
	b.WriteString("\r\n")
}

// getTermWidth 获取 stdout 终端宽度，失败时返回默认值。
var getTermWidth = func() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return tuiDefaultTerm
	}
	return w
}

// MTRTUIRender 将 MTR TUI 帧渲染到 w。
// 每帧重新获取终端宽度并计算自适应布局。
func MTRTUIRender(w io.Writer, header MTRTUIHeader, stats []trace.MTRHopStat) {
	mtrTUIRenderWithWidth(w, header, stats, getTermWidth())
}

// mtrTUIRenderWithWidth 是带可控宽度的内部渲染入口（测试用）。
func mtrTUIRenderWithWidth(w io.Writer, header MTRTUIHeader, stats []trace.MTRHopStat, termWidth int) {
	lo := computeLayout(termWidth)
	var b strings.Builder

	// 清屏（cursor home + erase）
	b.WriteString("\033[H\033[2J")

	statusStr := "Running"
	if header.Status == MTRTUIPaused {
		statusStr = "Paused"
	}

	// ── 信息行 1：NextTrace [version]  preferred API info（居中，纯文本） ──
	ver := header.Version
	if ver == "" {
		ver = "dev"
	}
	line1 := fmt.Sprintf("NextTrace [%s]", ver)
	if header.APIInfo != "" {
		line1 += "  " + header.APIInfo
	}
	// 截断 + 居中
	line1W := displayWidth(line1)
	if line1W > lo.termWidth {
		line1 = truncateByDisplayWidth(line1, lo.termWidth)
		line1W = displayWidth(line1)
	}
	if line1W < lo.termWidth {
		pad := (lo.termWidth - line1W) / 2
		line1 = strings.Repeat(" ", pad) + line1
	}
	tuiLine(&b, "%s", line1)

	// ── 信息行 2：srcHost (srcIP) -> dstName (dstIP)     RFC3339-time ──
	srcPart := header.SrcHost
	if srcPart != "" && header.SrcIP != "" && srcPart != header.SrcIP {
		srcPart = fmt.Sprintf("%s (%s)", srcPart, header.SrcIP)
	} else if header.SrcIP != "" {
		srcPart = header.SrcIP
	}

	dstPart := header.Target // 兜底
	if header.Domain != "" && header.TargetIP != "" && header.Domain != header.TargetIP {
		dstPart = fmt.Sprintf("%s (%s)", header.Domain, header.TargetIP)
	} else if header.TargetIP != "" {
		dstPart = header.TargetIP
	}

	var routeLine string
	if srcPart != "" {
		routeLine = fmt.Sprintf("%s -> %s", srcPart, dstPart)
	} else {
		routeLine = dstPart
	}
	timeStr := time.Now().Format("2006-01-02T15:04:05-0700")
	timeW := displayWidth(timeStr)
	routeW := displayWidth(routeLine)
	gap := lo.termWidth - routeW - timeW
	if gap < 2 {
		// 窄屏：截断 route 文本保证时间贴右
		maxRoute := lo.termWidth - timeW - 2
		if maxRoute < 1 {
			maxRoute = 1
		}
		routeLine = truncateByDisplayWidth(routeLine, maxRoute)
		gap = 2
	}
	tuiLine(&b, "%s%s%s", routeLine, strings.Repeat(" ", gap), timeStr)

	// ── 信息行 3：按键提示 + 显示模式 + 状态 ──
	modeNames := [4]string{"ASN", "City", "Owner", "Full"}
	modeLabel := "ASN"
	if header.DisplayMode >= 0 && header.DisplayMode < 4 {
		modeLabel = modeNames[header.DisplayMode]
	}
	nameLabel := "ptr"
	if header.NameMode == 1 {
		nameLabel = "ip"
	}
	tuiLine(&b, "Keys: q-quit  p-pause  SPACE-resume  r-reset  y-display(%s)  n-host(%s)          [%s] Round: %d",
		modeLabel, nameLabel, statusStr, header.Iteration)
	b.WriteString("\r\n") // 空行

	// ── 双层表头 ──
	renderDualHeader(&b, lo)

	// ── hop 数据行 ──
	lang := header.Lang
	if lang == "" {
		lang = "en"
	}
	nameMode := header.NameMode
	prevTTL := 0
	for _, s := range stats {
		hopPrefix := formatTUIHopPrefix(s.TTL, prevTTL)
		prevTTL = s.TTL

		host := formatMTRHostByMode(s, header.DisplayMode, nameMode, lang)
		renderDataRow(&b, lo, hopPrefix, host, s)

		// MPLS 多行显示：每个标签独占一行，位于 host 列区域
		if len(s.MPLS) > 0 {
			for _, mplsLabel := range s.MPLS {
				var mRow strings.Builder
				mRow.WriteString(strings.Repeat(" ", lo.prefixW+tuiPrefixGap))
				mRow.WriteString(fitLeft("  "+mplsLabel, lo.hostW))
				tuiLine(&b, "%s", mRow.String())
			}
		}
	}

	fmt.Fprint(w, b.String())
}

// renderDualHeader 渲染 mtr 风格双层分组表头。
//
//	第 1 行：左侧 "Host"，右侧分组 "Packets" 和 "Pings"
//	第 2 行：具体列名 Loss% Snt | Last Avg Best Wrst StDev
func renderDualHeader(b *strings.Builder, lo mtrTUILayout) {
	// -- 第 1 行 --
	prefix := strings.Repeat(" ", lo.prefixW+tuiPrefixGap)
	hostLabel := fitLeft("Host", lo.hostW)

	// "Packets" 覆盖 Loss+Snt，"Pings" 覆盖 5 个 RTT 列
	packetsW := lo.lossW + tuiMetricGap + lo.sntW
	pingsW := lo.lastW + tuiMetricGap + lo.avgW + tuiMetricGap + lo.bestW + tuiMetricGap + lo.wrstW + tuiMetricGap + lo.stdevW
	gap := strings.Repeat(" ", tuiHostGap)

	packetsLabel := centerIn("Packets", packetsW)
	pingsLabel := centerIn("Pings", pingsW)

	tuiLine(b, "%s%s%s%s %s", prefix, hostLabel, gap, packetsLabel, pingsLabel)

	// -- 第 2 行 --
	row := strings.Repeat(" ", lo.prefixW+tuiPrefixGap)
	row += padRight("", lo.hostW)
	row += strings.Repeat(" ", tuiHostGap)
	row += fitRight("Loss%", lo.lossW)
	row += strings.Repeat(" ", tuiMetricGap)
	row += fitRight("Snt", lo.sntW)
	row += strings.Repeat(" ", tuiMetricGap)
	row += fitRight("Last", lo.lastW)
	row += strings.Repeat(" ", tuiMetricGap)
	row += fitRight("Avg", lo.avgW)
	row += strings.Repeat(" ", tuiMetricGap)
	row += fitRight("Best", lo.bestW)
	row += strings.Repeat(" ", tuiMetricGap)
	row += fitRight("Wrst", lo.wrstW)
	row += strings.Repeat(" ", tuiMetricGap)
	row += fitRight("StDev", lo.stdevW)
	tuiLine(b, "%s", row)
}

// centerIn 将 s 在 width 宽度内居中，两侧空格填充。
func centerIn(s string, width int) string {
	w := runewidth.StringWidth(s)
	if w >= width {
		return runewidth.Truncate(s, width, "")
	}
	left := (width - w) / 2
	right := width - w - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// renderDataRow 渲染一行 hop 数据。
func renderDataRow(b *strings.Builder, lo mtrTUILayout, hopPrefix, host string, s trace.MTRHopStat) {
	var row strings.Builder

	// prefix
	row.WriteString(fitLeft(hopPrefix, lo.prefixW))
	row.WriteString(strings.Repeat(" ", tuiPrefixGap))

	// Host（CJK 安全截断 + 左对齐填充）
	row.WriteString(fitLeft(host, lo.hostW))
	row.WriteString(strings.Repeat(" ", tuiHostGap))

	// 指标列，右对齐
	row.WriteString(fitRight(formatLoss(s.Loss), lo.lossW))
	row.WriteString(strings.Repeat(" ", tuiMetricGap))
	row.WriteString(fitRight(fmt.Sprint(s.Snt), lo.sntW))
	row.WriteString(strings.Repeat(" ", tuiMetricGap))
	row.WriteString(fitRight(formatMs(s.Last), lo.lastW))
	row.WriteString(strings.Repeat(" ", tuiMetricGap))
	row.WriteString(fitRight(formatMs(s.Avg), lo.avgW))
	row.WriteString(strings.Repeat(" ", tuiMetricGap))
	row.WriteString(fitRight(formatMs(s.Best), lo.bestW))
	row.WriteString(strings.Repeat(" ", tuiMetricGap))
	row.WriteString(fitRight(formatMs(s.Wrst), lo.wrstW))
	row.WriteString(strings.Repeat(" ", tuiMetricGap))
	row.WriteString(fitRight(formatMs(s.StDev), lo.stdevW))

	tuiLine(b, "%s", row.String())
}

// MTRTUIRenderString 将 MTR TUI 帧渲染为字符串（方便测试）。
func MTRTUIRenderString(header MTRTUIHeader, stats []trace.MTRHopStat) string {
	var sb strings.Builder
	MTRTUIRender(&sb, header, stats)
	return sb.String()
}

// mtrTUIRenderStringWithWidth 宽度可控的渲染入口（测试用）。
func mtrTUIRenderStringWithWidth(header MTRTUIHeader, stats []trace.MTRHopStat, width int) string {
	var sb strings.Builder
	mtrTUIRenderWithWidth(&sb, header, stats, width)
	return sb.String()
}

// formatTUIHopPrefix 返回简化版跳数前缀：
//
//	"1."  新 TTL
//	"  "  同 TTL 多路径续行
func formatTUIHopPrefix(ttl, prevTTL int) string {
	if ttl == prevTTL {
		return "  "
	}
	return fmt.Sprintf("%d.", ttl)
}

// truncateStr 截断字符串到 maxLen 字节，超出时添加省略号。
// 对纯 ASCII 场景仍可使用；CJK 场景优先使用 truncateByDisplayWidth。
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "."
	}
	return s[:maxLen-1] + "."
}

// MTRTUIPrinter 返回一个可直接用作 MTROnSnapshot 的回调函数，
// 将帧渲染到 os.Stdout。
func MTRTUIPrinter(target, domain, targetIP, version string, startTime time.Time,
	srcHost, srcIP, lang, apiInfo string,
	isPaused func() bool, displayMode func() int, nameMode func() int) func(iteration int, stats []trace.MTRHopStat) {
	return func(iteration int, stats []trace.MTRHopStat) {
		status := MTRTUIRunning
		if isPaused != nil && isPaused() {
			status = MTRTUIPaused
		}
		mode := 0
		if displayMode != nil {
			mode = displayMode()
		}
		nm := 0
		if nameMode != nil {
			nm = nameMode()
		}
		MTRTUIRender(os.Stdout, MTRTUIHeader{
			Target:      target,
			StartTime:   startTime,
			Status:      status,
			Iteration:   iteration,
			Domain:      domain,
			TargetIP:    targetIP,
			Version:     version,
			SrcHost:     srcHost,
			SrcIP:       srcIP,
			Lang:        lang,
			DisplayMode: mode,
			NameMode:    nm,
			APIInfo:     apiInfo,
		}, stats)
	}
}
