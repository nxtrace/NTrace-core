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
	DisplayMode int    // 显示模式 0-4
	NameMode    int    // Host 基础显示 0=PTR/IP, 1=IP only
	ShowIPs     bool   // 是否显示 PTR+IP（nameMode=0 时生效）
	APIInfo     string // preferred API 信息（纯文本，可为空）
	DisableMPLS bool   // 是否隐藏 MPLS 行（运行时 toggle）
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
	tuiPrefixW     = 4 // 默认前缀宽度（TTL ≤ 99: "%2d. " = 4 列）
	tuiPrefixGap   = 0 // 前缀尾部已含空格
	tuiHostGap     = 2 // Host 与指标区之间最小间距
	tuiMetricGap   = 1 // 指标列之间间距
	tuiDefaultTerm = 120
	tuiTabStop     = 8 // tab 展开步长

	tuiLossDefault = 5
	tuiSntDefault  = 3
	tuiRTTDefault  = 7
	tuiHostDefault = 40
	tuiHostMin     = 8
	tuiLossMin     = 5
	tuiSntMin      = 3
	tuiRTTMin      = 5
)

// tuiPrefixWidthForMaxTTL 根据最大 TTL 值返回前缀列宽。
// 前缀格式 "%Nd. "，其中 N = max(2, digits(maxTTL))，列宽 = N + 2。
func tuiPrefixWidthForMaxTTL(maxTTL int) int {
	digits := 2
	if maxTTL >= 1000 {
		digits = 4
	} else if maxTTL >= 100 {
		digits = 3
	}
	return digits + 2 // ". " 后缀
}

// computeLayout 根据终端宽度和前缀宽度计算布局。
//
// prefixW 为 hop 前缀列宽，由 tuiPrefixWidthForMaxTTL 动态计算。
//
// 三阶段压缩策略：
//  1. 默认指标宽度，Host 取剩余空间
//  2. Host 降至 tuiHostMin，按比例压缩指标列
//  3. 极窄场景：循环缩减 Host（最低 1 列）直到 totalWidth ≤ termWidth
//
// 绝对下限 totalWidth = prefixW+prefixGap(0)+host(1)+hostGap(2)+7×1+6×1。
// 当 termWidth 低于下限时接受溢出——该宽度下终端本身已不可用。
// sntWidthForMax returns the display width needed for the given max Snt value.
// Minimum is tuiSntDefault (3).
func sntWidthForMax(maxSnt int) int {
	w := tuiSntDefault
	for v := 1000; maxSnt >= v; v *= 10 {
		w++
	}
	return w
}

func computeLayout(termWidth, prefixW, sntHint int) mtrTUILayout {
	if termWidth <= 0 {
		termWidth = tuiDefaultTerm
	}
	if prefixW <= 0 {
		prefixW = tuiPrefixW
	}
	sntW := tuiSntDefault
	if sntHint > sntW {
		sntW = sntHint
	}

	lo := mtrTUILayout{
		termWidth: termWidth,
		prefixW:   prefixW,
		lossW:     tuiLossDefault,
		sntW:      sntW,
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
		shrinkMetrics(metricsAvail, sntW)

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
// sntDefault 为当前 Snt 列目标宽度（可能因动态计算大于 tuiSntDefault）。
//
// 当 available 极小时，列宽可降至绝对下限 1，确保 computeLayout
// 的 phase-3 循环能把 totalWidth 压到 termWidth 以内。
func shrinkMetrics(available, sntDefault int) (lossW, sntW, lastW, avgW, bestW, wrstW, stdevW int) {
	if sntDefault < tuiSntDefault {
		sntDefault = tuiSntDefault
	}
	avail := available - 6*tuiMetricGap
	if avail < 7 {
		// 绝对下限：每列 1
		return 1, 1, 1, 1, 1, 1, 1
	}

	defaults := [7]int{tuiLossDefault, sntDefault, tuiRTTDefault, tuiRTTDefault, tuiRTTDefault, tuiRTTDefault, tuiRTTDefault}
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

// ---------------------------------------------------------------------------
// Tab 感知宽度辅助
// ---------------------------------------------------------------------------

// displayWidthWithTabs 返回包含 tab 的字符串在终端上的显示宽度。
// tabStop 为 tab 停靠间隔（通常为 8）。
func displayWidthWithTabs(s string, tabStop int) int {
	col := 0
	for _, r := range s {
		if r == '\t' {
			col = ((col / tabStop) + 1) * tabStop
		} else {
			col += runewidth.RuneWidth(r)
		}
	}
	return col
}

// truncateWithTabs 将包含 tab 的字符串截断到不超过 maxW 显示列。
func truncateWithTabs(s string, maxW int, tabStop int) string {
	if maxW <= 0 {
		return ""
	}
	col := 0
	var result strings.Builder
	for _, r := range s {
		var nextCol int
		if r == '\t' {
			nextCol = ((col / tabStop) + 1) * tabStop
		} else {
			nextCol = col + runewidth.RuneWidth(r)
		}
		if nextCol > maxW {
			break
		}
		result.WriteRune(r)
		col = nextCol
	}
	return result.String()
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
	lo := buildMTRTUILayout(stats, termWidth)
	var b strings.Builder

	writeMTRTUIFramePrefix(&b)
	renderMTRTUIHeader(&b, header, lo.termWidth)
	renderDualHeader(&b, lo)
	renderMTRTUIRows(&b, header, stats, lo)
	fmt.Fprint(w, b.String())
}

func buildMTRTUILayout(stats []trace.MTRHopStat, termWidth int) mtrTUILayout {
	maxTTL, maxSnt := scanMTRTUIStats(stats)
	prefixW := tuiPrefixWidthForMaxTTL(maxTTL)
	return computeLayout(termWidth, prefixW, sntWidthForMax(maxSnt))
}

func scanMTRTUIStats(stats []trace.MTRHopStat) (int, int) {
	maxTTL := 0
	maxSnt := 0
	for _, s := range stats {
		if s.TTL > maxTTL {
			maxTTL = s.TTL
		}
		if s.Snt > maxSnt {
			maxSnt = s.Snt
		}
	}
	return maxTTL, maxSnt
}

func writeMTRTUIFramePrefix(b *strings.Builder) {
	b.WriteString("\033[H\033[2J")
}

func renderMTRTUIHeader(b *strings.Builder, header MTRTUIHeader, termWidth int) {
	tuiLine(b, "%s", buildMTRTUITitleLine(header, termWidth))
	tuiLine(b, "%s", buildMTRTUIRouteLine(header, termWidth, time.Now()))
	tuiLine(b, "%s", buildMTRTUIControlsLine(header, termWidth))
}

func buildMTRTUITitleLine(header MTRTUIHeader, termWidth int) string {
	titlePart, apiPart := resolveMTRTUITitleParts(header)
	line := titlePart + apiPart
	lineW := displayWidth(line)
	if lineW > termWidth {
		line = truncateByDisplayWidth(line, termWidth)
		lineW = displayWidth(line)
		titleW := displayWidth(titlePart)
		if lineW <= titleW {
			titlePart = line
			apiPart = ""
		} else {
			apiPart = line[len(titlePart):]
		}
	}
	pad := 0
	if lineW < termWidth {
		pad = (termWidth - lineW) / 2
	}
	return strings.Repeat(" ", pad) + mtrTUITitleColor(titlePart) + apiPart
}

func resolveMTRTUITitleParts(header MTRTUIHeader) (string, string) {
	ver := header.Version
	if ver == "" {
		ver = "dev"
	}
	titlePart := fmt.Sprintf("NextTrace [%s]", ver)
	if header.APIInfo == "" {
		return titlePart, ""
	}
	return titlePart, "  " + header.APIInfo
}

func buildMTRTUIRouteLine(header MTRTUIHeader, termWidth int, now time.Time) string {
	routeLine := buildMTRTUIRouteText(header)
	timeStr := now.Format("2006-01-02T15:04:05-0700")
	timeW := displayWidth(timeStr)
	gap := termWidth - displayWidth(routeLine) - timeW
	if gap < 2 {
		maxRoute := termWidth - timeW - 2
		if maxRoute < 1 {
			maxRoute = 1
		}
		routeLine = truncateByDisplayWidth(routeLine, maxRoute)
		gap = 2
	}
	return mtrTUIRouteColor(routeLine) + strings.Repeat(" ", gap) + mtrTUITimeColor(timeStr)
}

func buildMTRTUIRouteText(header MTRTUIHeader) string {
	srcPart := resolveMTRTUISourceLabel(header)
	dstPart := resolveMTRTUIDestinationLabel(header)
	if srcPart == "" {
		return dstPart
	}
	return fmt.Sprintf("%s -> %s", srcPart, dstPart)
}

func resolveMTRTUISourceLabel(header MTRTUIHeader) string {
	switch {
	case header.SrcHost != "" && header.SrcIP != "" && header.SrcHost != header.SrcIP:
		return fmt.Sprintf("%s (%s)", header.SrcHost, header.SrcIP)
	case header.SrcIP != "":
		return header.SrcIP
	default:
		return header.SrcHost
	}
}

func resolveMTRTUIDestinationLabel(header MTRTUIHeader) string {
	switch {
	case header.Domain != "" && header.TargetIP != "" && header.Domain != header.TargetIP:
		return fmt.Sprintf("%s (%s)", header.Domain, header.TargetIP)
	case header.TargetIP != "":
		return header.TargetIP
	case header.Target != "":
		return header.Target
	default:
		return ""
	}
}

func buildMTRTUIControlsLine(header MTRTUIHeader, termWidth int) string {
	const keysPrefix = "Keys:  "
	keyLine := strings.Join(buildMTRTUIKeyItems(header), "  ")
	statusText := mtrTUIStatusText(header.Status)
	statusTag := mtrTUIStatusColor("[" + statusText + "]")
	pad := termWidth - displayWidth(keysPrefix) - displayWidth(keyLine) - len("["+statusText+"]")
	if pad < 2 {
		pad = 2
	}
	return keysPrefix + keyLine + strings.Repeat(" ", pad) + statusTag
}

func buildMTRTUIKeyItems(header MTRTUIHeader) []string {
	return []string{
		mtrTUIKeyHiColor("Q") + "uit",
		mtrTUIKeyHiColor("P") + "ause",
		mtrTUIKeyHiColor("Space") + "-resume",
		mtrTUIKeyHiColor("R") + "eset",
		mtrTUIKeyHiColor("Y") + "-display(" + mtrTUIDisplayModeLabel(header.DisplayMode) + ")",
		mtrTUIKeyHiColor("N") + "-host(" + mtrTUINameModeLabel(header.NameMode, header.ShowIPs) + ")",
		mtrTUIKeyHiColor("E") + "-mpls(" + mtrTUIMPLSLabel(header.DisableMPLS) + ")",
	}
}

func mtrTUIStatusText(status MTRTUIStatus) string {
	if status == MTRTUIPaused {
		return "Paused"
	}
	return "Running"
}

func mtrTUIDisplayModeLabel(mode int) string {
	modeNames := [5]string{"IP/PTR", "ASN", "City", "Owner", "Full"}
	if mode >= 0 && mode < len(modeNames) {
		return modeNames[mode]
	}
	return modeNames[0]
}

func mtrTUINameModeLabel(nameMode int, showIPs bool) string {
	if nameMode == 1 {
		return "ip"
	}
	if showIPs {
		return "ptr+ip"
	}
	return "ptr"
}

func mtrTUIMPLSLabel(disabled bool) string {
	if disabled {
		return "show"
	}
	return "hide"
}

func renderMTRTUIRows(b *strings.Builder, header MTRTUIHeader, stats []trace.MTRHopStat, lo mtrTUILayout) {
	allParts := buildTUIHostPartSet(stats, header)
	asnW := computeTUIASNWidthFromParts(allParts)
	prevTTL := 0
	for i, s := range stats {
		hopPrefix := formatTUIHopPrefix(s.TTL, prevTTL, lo.prefixW)
		prevTTL = s.TTL
		renderDataRow(b, lo, hopPrefix, formatTUIHost(allParts[i], asnW), s)
		renderMTRTUIMPLSRows(b, lo, s.MPLS, header.DisableMPLS)
	}
}

func buildTUIHostPartSet(stats []trace.MTRHopStat, header MTRTUIHeader) []mtrHostParts {
	lang := header.Lang
	if lang == "" {
		lang = "en"
	}
	allParts := make([]mtrHostParts, len(stats))
	for i, s := range stats {
		allParts[i] = buildTUIHostParts(s, header.DisplayMode, header.NameMode, lang, header.ShowIPs)
	}
	return allParts
}

func renderMTRTUIMPLSRows(b *strings.Builder, lo mtrTUILayout, labels []string, disabled bool) {
	if disabled || len(labels) == 0 {
		return
	}
	for _, label := range labels {
		var row strings.Builder
		row.WriteString(strings.Repeat(" ", lo.prefixW+tuiPrefixGap))
		row.WriteString(mtrTUIMPLSColor(fitLeft("  "+label, lo.hostW)))
		tuiLine(b, "%s", row.String())
	}
}

func computeTUIASNWidth(stats []trace.MTRHopStat, mode int, nameMode int, lang string, showIPs bool) int {
	allParts := make([]mtrHostParts, len(stats))
	for i, s := range stats {
		allParts[i] = buildTUIHostParts(s, mode, nameMode, lang, showIPs)
	}
	return computeTUIASNWidthFromParts(allParts)
}

func computeTUIASNWidthFromParts(allParts []mtrHostParts) int {
	maxW := 0
	for _, parts := range allParts {
		if parts.waiting || parts.asn == "" {
			continue
		}
		if w := displayWidth(parts.asn); w > maxW {
			maxW = w
		}
	}
	if maxW == 0 {
		return 0
	}
	minW := displayWidth("AS???")
	if maxW < minW {
		return minW
	}
	return maxW
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

	tuiLine(b, "%s%s%s%s %s",
		prefix,
		mtrTUIHeaderColor(hostLabel),
		gap,
		mtrTUIHeaderColor(packetsLabel),
		mtrTUIHeaderColor(pingsLabel))

	// -- 第 2 行 --
	row := strings.Repeat(" ", lo.prefixW+tuiPrefixGap)
	row += padRight("", lo.hostW)
	row += strings.Repeat(" ", tuiHostGap)
	row += mtrTUIHeaderColor(fitRight("Loss%", lo.lossW))
	row += strings.Repeat(" ", tuiMetricGap)
	row += mtrTUIHeaderColor(fitRight("Snt", lo.sntW))
	row += strings.Repeat(" ", tuiMetricGap)
	row += mtrTUIHeaderColor(fitRight("Last", lo.lastW))
	row += strings.Repeat(" ", tuiMetricGap)
	row += mtrTUIHeaderColor(fitRight("Avg", lo.avgW))
	row += strings.Repeat(" ", tuiMetricGap)
	row += mtrTUIHeaderColor(fitRight("Best", lo.bestW))
	row += strings.Repeat(" ", tuiMetricGap)
	row += mtrTUIHeaderColor(fitRight("Wrst", lo.wrstW))
	row += strings.Repeat(" ", tuiMetricGap)
	row += mtrTUIHeaderColor(fitRight("StDev", lo.stdevW))
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
//
// 左侧为 prefix+hostText（含 tab），填充到 metricsStart 后拼接指标列，
// 保证右侧指标列始终键齐。
func renderDataRow(b *strings.Builder, lo mtrTUILayout, hopPrefix, host string, s trace.MTRHopStat) {
	left := hopPrefix + host
	leftW := displayWidthWithTabs(left, tuiTabStop)

	// 截断：确保 left 不超过 metricsStart - 1（至少保留 1 列间距）
	maxLeft := lo.metricsStart - 1
	if maxLeft < 1 {
		maxLeft = 1
	}
	if leftW > maxLeft {
		left = truncateWithTabs(left, maxLeft, tuiTabStop)
		leftW = displayWidthWithTabs(left, tuiTabStop)
	}

	var row strings.Builder
	waiting := isWaitingHopStat(s)
	leftColored := mtrTUIHostColor(left)
	if strings.HasPrefix(left, hopPrefix) {
		hostPart := left[len(hopPrefix):]
		hostSty := mtrTUIHostColor
		if waiting {
			hostSty = mtrTUIWaitColor
		}
		leftColored = mtrTUIHopColor(hopPrefix) + hostSty(hostPart)
	}
	row.WriteString(leftColored)
	// 填充空格到 metricsStart
	if gap := lo.metricsStart - leftW; gap > 0 {
		row.WriteString(strings.Repeat(" ", gap))
	}

	// 指标列，右对齐
	m := formatMTRMetricStrings(s)
	lossCell := fitRight(m.loss, lo.lossW)
	sntCell := fitRight(m.snt, lo.sntW)
	lossCell, sntCell = mtrColorPacketsByLoss(lossCell, sntCell, s.Loss, waiting)

	row.WriteString(lossCell)
	row.WriteString(strings.Repeat(" ", tuiMetricGap))
	row.WriteString(sntCell)
	row.WriteString(strings.Repeat(" ", tuiMetricGap))
	row.WriteString(fitRight(m.last, lo.lastW))
	row.WriteString(strings.Repeat(" ", tuiMetricGap))
	row.WriteString(fitRight(m.avg, lo.avgW))
	row.WriteString(strings.Repeat(" ", tuiMetricGap))
	row.WriteString(fitRight(m.best, lo.bestW))
	row.WriteString(strings.Repeat(" ", tuiMetricGap))
	row.WriteString(fitRight(m.wrst, lo.wrstW))
	row.WriteString(strings.Repeat(" ", tuiMetricGap))
	row.WriteString(fitRight(m.stdev, lo.stdevW))

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

// formatTUIHopPrefix 返回紧凑版跳数前缀，宽度由 prefixW 控制：
//
//	prefixW=4: " 1. "  新 TTL / "    " 续行
//	prefixW=5: "  1. " 新 TTL / "     " 续行
func formatTUIHopPrefix(ttl, prevTTL, prefixW int) string {
	if ttl == prevTTL {
		return strings.Repeat(" ", prefixW)
	}
	digitW := prefixW - 2 // ". " 后缀占 2
	if digitW < 2 {
		digitW = 2
	}
	return fmt.Sprintf("%*d. ", digitW, ttl)
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
	srcHost, srcIP, lang string, apiInfo func() string, showIPs bool,
	isPaused func() bool, displayMode func() int, nameMode func() int, isMPLSDisabled func() bool) func(iteration int, stats []trace.MTRHopStat) {
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
		noMPLS := false
		if isMPLSDisabled != nil {
			noMPLS = isMPLSDisabled()
		}
		headerAPIInfo := ""
		if apiInfo != nil {
			headerAPIInfo = apiInfo()
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
			ShowIPs:     showIPs,
			APIInfo:     headerAPIInfo,
			DisableMPLS: noMPLS,
		}, stats)
	}
}
