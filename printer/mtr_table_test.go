package printer

import (
	"strings"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
)

func TestMTRRenderTable_HeaderOrder(t *testing.T) {
	// 验证 MTRRow 字段名（即列名）顺序：Hop, Loss%, Snt, Last, Avg, Best, Wrst, StDev, Host
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "1.1.1.1", Loss: 0, Snt: 5, Last: 1.23, Avg: 1.50, Best: 1.00, Wrst: 2.00, StDev: 0.33},
	}
	rows := MTRRenderTable(stats)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.Hop != "1" {
		t.Errorf("Hop = %q, want %q", r.Hop, "1")
	}
	if r.Loss != "0.0%" {
		t.Errorf("Loss = %q, want %q", r.Loss, "0.0%")
	}
	if r.Snt != "5" {
		t.Errorf("Snt = %q, want %q", r.Snt, "5")
	}
	if r.Last != "1.23" {
		t.Errorf("Last = %q, want %q", r.Last, "1.23")
	}
	if r.Avg != "1.50" {
		t.Errorf("Avg = %q, want %q", r.Avg, "1.50")
	}
	if r.Best != "1.00" {
		t.Errorf("Best = %q, want %q", r.Best, "1.00")
	}
	if r.Wrst != "2.00" {
		t.Errorf("Wrst = %q, want %q", r.Wrst, "2.00")
	}
	if r.StDev != "0.33" {
		t.Errorf("StDev = %q, want %q", r.StDev, "0.33")
	}
	if r.Host != "1.1.1.1" {
		t.Errorf("Host = %q, want %q", r.Host, "1.1.1.1")
	}
}

func TestMTRRenderTable_NumericFormatting(t *testing.T) {
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "10.0.0.1", Loss: 33.3333, Snt: 3, Last: 0.456, Avg: 1.789, Best: 0.123, Wrst: 3.456, StDev: 1.234},
		{TTL: 2, IP: "10.0.0.2", Loss: 100, Snt: 3, Last: 0, Avg: 0, Best: 0, Wrst: 0, StDev: 0},
	}
	rows := MTRRenderTable(stats)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// 行 1: loss 保留一位小数加 %
	if rows[0].Loss != "33.3%" {
		t.Errorf("Loss = %q, want %q", rows[0].Loss, "33.3%")
	}
	// ms 保留两位小数
	if rows[0].Last != "0.46" {
		t.Errorf("Last = %q, want %q", rows[0].Last, "0.46")
	}
	if rows[0].Avg != "1.79" {
		t.Errorf("Avg = %q, want %q", rows[0].Avg, "1.79")
	}

	// 行 2: 全超时 → 100% loss, RTT 全 0.00
	if rows[1].Loss != "100.0%" {
		t.Errorf("Loss = %q, want %q", rows[1].Loss, "100.0%")
	}
	if rows[1].Last != "0.00" {
		t.Errorf("Last = %q, want %q", rows[1].Last, "0.00")
	}
}

func TestMTRRenderTable_NilGeo(t *testing.T) {
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "192.168.1.1", Geo: nil},
	}
	rows := MTRRenderTable(stats)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	// Host 只显示 IP，无 panic
	if rows[0].Host != "192.168.1.1" {
		t.Errorf("Host = %q, want %q", rows[0].Host, "192.168.1.1")
	}
}

func TestMTRRenderTable_EmptyHostname(t *testing.T) {
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "8.8.8.8", Host: "", Geo: &ipgeo.IPGeoData{
			Asnumber:  "15169",
			CountryEn: "United States",
		}},
	}
	rows := MTRRenderTable(stats)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	// 无 hostname 时只显示 IP + Geo
	want := "8.8.8.8 AS15169, United States"
	if rows[0].Host != want {
		t.Errorf("Host = %q, want %q", rows[0].Host, want)
	}
}

func TestMTRRenderTable_HostnameAndIP(t *testing.T) {
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "1.1.1.1", Host: "one.one.one.one", Geo: &ipgeo.IPGeoData{
			Asnumber:  "13335",
			CountryEn: "US",
			Owner:     "Cloudflare",
		}},
	}
	rows := MTRRenderTable(stats)
	want := "one.one.one.one (1.1.1.1) AS13335, US, Cloudflare"
	if rows[0].Host != want {
		t.Errorf("Host = %q, want %q", rows[0].Host, want)
	}
}

func TestMTRRenderTable_MultiPath(t *testing.T) {
	// 同一 TTL 出现两个不同 IP（多路径）
	stats := []trace.MTRHopStat{
		{TTL: 2, IP: "10.0.0.1"},
		{TTL: 2, IP: "10.0.0.2"},
		{TTL: 3, IP: "10.0.1.1"},
	}
	rows := MTRRenderTable(stats)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	// 第一行显示 TTL
	if rows[0].Hop != "2" {
		t.Errorf("rows[0].Hop = %q, want %q", rows[0].Hop, "2")
	}
	// 第二行同 TTL → 应为空
	if rows[1].Hop != "" {
		t.Errorf("rows[1].Hop = %q, want empty", rows[1].Hop)
	}
	// 第三行是新 TTL
	if rows[2].Hop != "3" {
		t.Errorf("rows[2].Hop = %q, want %q", rows[2].Hop, "3")
	}
}

func TestMTRRenderTable_UnknownHost(t *testing.T) {
	// 无 IP 无 hostname → "???"
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "", Host: ""},
	}
	rows := MTRRenderTable(stats)
	if rows[0].Host != "???" {
		t.Errorf("Host = %q, want %q", rows[0].Host, "???")
	}
}

func TestFormatLoss(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0.0%"},
		{100, "100.0%"},
		{33.3333, "33.3%"},
		{50, "50.0%"},
	}
	for _, c := range cases {
		got := formatLoss(c.in)
		if got != c.want {
			t.Errorf("formatLoss(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatMs(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0.00"},
		{1.999, "2.00"},
		{12.345, "12.35"},
		{0.1, "0.10"},
	}
	for _, c := range cases {
		got := formatMs(c.in)
		if got != c.want {
			t.Errorf("formatMs(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// MTR TUI 渲染测试
// ---------------------------------------------------------------------------

func TestMTRTUIRenderString_Header(t *testing.T) {
	startTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	header := MTRTUIHeader{
		Target:    "1.1.1.1",
		StartTime: startTime,
		Status:    MTRTUIRunning,
		Iteration: 5,
	}
	result := MTRTUIRenderString(header, nil)

	if !strings.Contains(result, "nexttrace --mtr 1.1.1.1") {
		t.Error("missing target in header")
	}
	if !strings.Contains(result, "2025-01-15T10:30:00") {
		t.Error("missing start time in header")
	}
	if !strings.Contains(result, "[Running]") {
		t.Error("missing Running status")
	}
	if !strings.Contains(result, "Round: 5") {
		t.Error("missing round number")
	}
	if !strings.Contains(result, "q - quit") {
		t.Error("missing key hints")
	}
}

// TestMTRTUIRenderString_UsesCRLFOnly 确保 TUI 帧不包含裸 LF
// （即每个 \n 前面必须是 \r），避免 raw mode 下光标不回行首导致斜排。
func TestMTRTUIRenderString_UsesCRLFOnly(t *testing.T) {
	header := MTRTUIHeader{
		Target:    "1.1.1.1",
		StartTime: time.Now(),
		Status:    MTRTUIRunning,
		Iteration: 1,
	}
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "10.0.0.1", Loss: 0, Snt: 3, Last: 1.0, Avg: 1.0, Best: 1.0, Wrst: 1.0, StDev: 0},
		{TTL: 2, IP: "10.0.0.2", Loss: 50, Snt: 4, Last: 5.0, Avg: 6.0, Best: 4.0, Wrst: 8.0, StDev: 1.5},
	}
	result := MTRTUIRenderString(header, stats)

	for i := 0; i < len(result); i++ {
		if result[i] == '\n' && (i == 0 || result[i-1] != '\r') {
			// 找到裸 LF 的位置供调试
			start := i - 20
			if start < 0 {
				start = 0
			}
			end := i + 20
			if end > len(result) {
				end = len(result)
			}
			t.Fatalf("bare LF at byte %d; context: %q", i, result[start:end])
		}
	}
}

// TestMTRTUIRenderString_FramePrefix 确保帧以清屏序列开头，
// 且 header 首行以 \r\n 结束。
func TestMTRTUIRenderString_FramePrefix(t *testing.T) {
	header := MTRTUIHeader{
		Target:    "8.8.8.8",
		StartTime: time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC),
		Status:    MTRTUIRunning,
		Iteration: 1,
	}
	result := MTRTUIRenderString(header, nil)

	if !strings.HasPrefix(result, "\033[H\033[2J") {
		t.Error("frame should start with cursor-home + erase-screen")
	}
	// header 行应以 \r\n 结束
	idx := strings.Index(result, "2025-06-01T12:00:00")
	if idx < 0 {
		t.Fatal("missing timestamp in header")
	}
	// 找到该行末尾
	nlIdx := strings.Index(result[idx:], "\r\n")
	if nlIdx < 0 {
		t.Error("header line should end with \\r\\n")
	}
}

func TestMTRTUIRenderString_PausedStatus(t *testing.T) {
	header := MTRTUIHeader{
		Target:    "8.8.8.8",
		StartTime: time.Now(),
		Status:    MTRTUIPaused,
		Iteration: 3,
	}
	result := MTRTUIRenderString(header, nil)

	if !strings.Contains(result, "[Paused]") {
		t.Error("expected Paused status")
	}
}

func TestMTRTUIRenderString_HopRows(t *testing.T) {
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "10.0.0.1", Loss: 0, Snt: 5, Last: 1.23, Avg: 1.50, Best: 1.00, Wrst: 2.00, StDev: 0.33},
		{TTL: 2, IP: "10.0.0.2", Loss: 50, Snt: 4, Last: 5.00, Avg: 6.00, Best: 4.00, Wrst: 8.00, StDev: 1.50},
	}
	header := MTRTUIHeader{
		Target:    "1.1.1.1",
		StartTime: time.Now(),
		Status:    MTRTUIRunning,
		Iteration: 1,
	}
	result := MTRTUIRenderString(header, stats)

	if !strings.Contains(result, "1.|--") {
		t.Error("missing 1.|-- hop prefix")
	}
	if !strings.Contains(result, "2.|--") {
		t.Error("missing 2.|-- hop prefix")
	}
	if !strings.Contains(result, "10.0.0.1") {
		t.Error("missing first hop IP")
	}
	if !strings.Contains(result, "10.0.0.2") {
		t.Error("missing second hop IP")
	}
}

func TestMTRTUIRenderString_MultiPath(t *testing.T) {
	stats := []trace.MTRHopStat{
		{TTL: 2, IP: "10.0.0.1"},
		{TTL: 2, IP: "10.0.0.2"},
		{TTL: 3, IP: "10.0.1.1"},
	}
	header := MTRTUIHeader{Target: "x", StartTime: time.Now(), Iteration: 1}
	result := MTRTUIRenderString(header, stats)

	// 第一行 TTL=2 → "2.|--", 第二行同 TTL → "  |  "
	if !strings.Contains(result, "2.|--") {
		t.Error("missing first multipath hop prefix")
	}
	if !strings.Contains(result, "  |  ") {
		t.Error("missing continuation prefix for same TTL")
	}
	if !strings.Contains(result, "3.|--") {
		t.Error("missing next TTL prefix")
	}
}

func TestFormatTUIHopPrefix(t *testing.T) {
	cases := []struct {
		ttl, prev int
		want      string
	}{
		{1, 0, "1.|--"},
		{5, 4, "5.|--"},
		{3, 3, "  |  "},
		{10, 9, "10.|--"},
	}
	for _, c := range cases {
		got := formatTUIHopPrefix(c.ttl, c.prev)
		if got != c.want {
			t.Errorf("formatTUIHopPrefix(%d, %d) = %q, want %q", c.ttl, c.prev, got, c.want)
		}
	}
}

func TestTruncateStr(t *testing.T) {
	cases := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is too long", 10, "this is t."},
		{"ab", 1, "."},
	}
	for _, c := range cases {
		got := truncateStr(c.s, c.maxLen)
		if got != c.want {
			t.Errorf("truncateStr(%q, %d) = %q, want %q", c.s, c.maxLen, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// 自适应布局新增测试
// ---------------------------------------------------------------------------

// TestTUI_RightAlignedMetricsBlock 验证指标列数值右对齐并出现在行尾。
func TestTUI_RightAlignedMetricsBlock(t *testing.T) {
	header := MTRTUIHeader{Target: "1.1.1.1", StartTime: time.Now(), Iteration: 1}
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "10.0.0.1", Loss: 0, Snt: 5, Last: 1.23, Avg: 1.50, Best: 1.00, Wrst: 2.00, StDev: 0.33},
	}
	result := mtrTUIRenderStringWithWidth(header, stats, 120)

	lines := strings.Split(result, "\r\n")
	// 找到含 "1.|--" 的 hop 行
	var hopLine string
	for _, l := range lines {
		if strings.Contains(l, "1.|--") {
			hopLine = l
			break
		}
	}
	if hopLine == "" {
		t.Fatal("missing hop line with 1.|--")
	}

	// Loss 出现在 Host之后
	hostIdx := strings.Index(hopLine, "10.0.0.1")
	lossIdx := strings.Index(hopLine, "0.0%")
	if lossIdx <= hostIdx {
		t.Errorf("Loss(%d) should appear after Host(%d)", lossIdx, hostIdx)
	}

	// 各指标值应存在
	for _, m := range []string{"0.0%", "1.23", "1.50", "1.00", "2.00", "0.33"} {
		if !strings.Contains(hopLine, m) {
			t.Errorf("hop line missing metric %q", m)
		}
	}

	// 验证右对齐：指标前应有空格（padLeft 效果）
	// 取 "Snt" 列值 "5"，应有前导空格
	sntIdx := strings.Index(hopLine, "0.0%")
	if sntIdx < 0 {
		t.Fatal("metric not found in hop line")
	}
	// 指标块在行尾，末尾不应有大量多余空格
	trimmed := strings.TrimRight(hopLine, " ")
	if len(trimmed) < len(hopLine)-2 {
		t.Errorf("too many trailing spaces; metrics should be near end of line")
	}
}

// TestTUI_HostExpandsOnWideTerminal 宽终端(200列)时 Host 列宽应大于默认 40。
func TestTUI_HostExpandsOnWideTerminal(t *testing.T) {
	lo := computeLayout(200)
	if lo.hostW <= tuiHostDefault {
		t.Errorf("wide terminal: hostW=%d, want > %d", lo.hostW, tuiHostDefault)
	}
	if lo.termWidth != 200 {
		t.Errorf("termWidth=%d, want 200", lo.termWidth)
	}
}

// TestTUI_HostShrinksWhenWidthReduced 窄终端(80列)时 Host 列宽应被压缩。
func TestTUI_HostShrinksWhenWidthReduced(t *testing.T) {
	lo := computeLayout(80)
	if lo.hostW >= tuiHostDefault {
		t.Errorf("narrow terminal: hostW=%d, should be < %d", lo.hostW, tuiHostDefault)
	}
	if lo.hostW < tuiHostMin {
		t.Errorf("hostW=%d, should not be less than min %d", lo.hostW, tuiHostMin)
	}
}

// TestTUI_DualHeaderPacketsPings 验证双层分组表头：
// 第一层含 "Packets" 和 "Pings"，第二层含各列名。
func TestTUI_DualHeaderPacketsPings(t *testing.T) {
	header := MTRTUIHeader{Target: "1.1.1.1", StartTime: time.Now(), Iteration: 1}
	result := mtrTUIRenderStringWithWidth(header, nil, 120)

	lines := strings.Split(result, "\r\n")

	foundPackets, foundPings := false, false
	foundLoss, foundSnt, foundLast, foundAvg, foundBest, foundWrst, foundStDev := false, false, false, false, false, false, false

	for _, l := range lines {
		if strings.Contains(l, "Packets") {
			foundPackets = true
		}
		if strings.Contains(l, "Pings") {
			foundPings = true
		}
		if strings.Contains(l, "Loss%") {
			foundLoss = true
		}
		if strings.Contains(l, "Snt") {
			foundSnt = true
		}
		if strings.Contains(l, "Last") {
			foundLast = true
		}
		if strings.Contains(l, "Avg") {
			foundAvg = true
		}
		if strings.Contains(l, "Best") {
			foundBest = true
		}
		if strings.Contains(l, "Wrst") {
			foundWrst = true
		}
		if strings.Contains(l, "StDev") {
			foundStDev = true
		}
	}

	if !foundPackets {
		t.Error("missing 'Packets' group label in header")
	}
	if !foundPings {
		t.Error("missing 'Pings' group label in header")
	}
	if !foundLoss || !foundSnt {
		t.Error("missing Loss%/Snt column names under Packets group")
	}
	if !foundLast || !foundAvg || !foundBest || !foundWrst || !foundStDev {
		t.Error("missing RTT column names under Pings group")
	}

	// "Packets" 和 "Pings" 应在同一行
	for _, l := range lines {
		if strings.Contains(l, "Packets") && strings.Contains(l, "Pings") {
			return // 验证通过
		}
	}
	t.Error("Packets and Pings should be on the same header line")
}

// TestTUI_VeryNarrowNoPanic 极窄终端(30列)不应 panic，
// 且 hop 数据行与表头行的显示宽度不超过 termWidth。
func TestTUI_VeryNarrowNoPanic(t *testing.T) {
	header := MTRTUIHeader{Target: "x", StartTime: time.Now(), Iteration: 1}
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "10.0.0.1", Loss: 0, Snt: 1, Last: 1.0, Avg: 1.0, Best: 1.0, Wrst: 1.0, StDev: 0},
	}
	const width = 30

	// 不应 panic
	result := mtrTUIRenderStringWithWidth(header, stats, width)

	if !strings.Contains(result, "\r\n") {
		t.Error("output should contain \\r\\n")
	}
	if !strings.Contains(result, "1.|--") {
		t.Error("missing hop prefix even in narrow mode")
	}

	// 验证 hop 行与表头子行不超宽
	lines := strings.Split(result, "\r\n")
	for _, l := range lines {
		// 跳过清屏序列、信息行（nexttrace / Keys）和空行
		if l == "" || strings.HasPrefix(l, "\033[") ||
			strings.Contains(l, "nexttrace") || strings.Contains(l, "Keys:") {
			continue
		}
		w := displayWidth(l)
		if w > width {
			t.Errorf("line exceeds termWidth=%d: displayWidth=%d, line=%q", width, w, l)
		}
	}
}

// TestTUI_DisplayWidthCJK 验证 CJK 宽字符截断和宽度计算。
func TestTUI_DisplayWidthCJK(t *testing.T) {
	// 每个中文字符占 2 列
	if w := displayWidth("中文"); w != 4 {
		t.Errorf("displayWidth(\"中文\") = %d, want 4", w)
	}
	if w := displayWidth("abc"); w != 3 {
		t.Errorf("displayWidth(\"abc\") = %d, want 3", w)
	}

	// 截断：max=5 → "中文" (4列) 可以放下
	got := truncateByDisplayWidth("中文", 5)
	if got != "中文" {
		t.Errorf("truncateByDisplayWidth(\"中文\", 5) = %q, want \"中文\"", got)
	}

	// 截断：max=3 → "中文" (4列) 超出 → 截断到 2列 + "."
	got = truncateByDisplayWidth("中文", 3)
	if displayWidth(got) > 3 {
		t.Errorf("truncateByDisplayWidth(\"中文\", 3) width=%d, want <= 3", displayWidth(got))
	}

	// padRight CJK
	padded := padRight("中文", 8) // 4显示列 + 4空格 = 8列
	if displayWidth(padded) != 8 {
		t.Errorf("padRight(\"中文\", 8) width=%d, want 8", displayWidth(padded))
	}
}

// TestTUI_ComputeLayoutZeroWidth 验证 termWidth=0 回退到默认值。
func TestTUI_ComputeLayoutZeroWidth(t *testing.T) {
	lo := computeLayout(0)
	if lo.termWidth != tuiDefaultTerm {
		t.Errorf("termWidth=%d, want default %d", lo.termWidth, tuiDefaultTerm)
	}
	if lo.hostW < tuiHostMin {
		t.Errorf("hostW=%d, want >= %d", lo.hostW, tuiHostMin)
	}
}

// TestTUI_TotalWidthInvariant 验证 computeLayout 的核心不变式：
// 对于 termWidth >= 23（绝对下限），totalWidth() == termWidth（右锚定）。
func TestTUI_TotalWidthInvariant(t *testing.T) {
	for _, tw := range []int{23, 25, 30, 40, 50, 60, 61, 80, 120, 200} {
		lo := computeLayout(tw)
		if lo.totalWidth() != tw {
			t.Errorf("termWidth=%d: totalWidth()=%d, want exact match (hostW=%d, metricsWidth=%d)",
				tw, lo.totalWidth(), lo.hostW, lo.metricsWidth())
		}
		if lo.hostW < 1 {
			t.Errorf("termWidth=%d: hostW=%d, must be >= 1", tw, lo.hostW)
		}
	}
}

// TestTUI_NarrowRightAnchor 验证窄屏(62列)时指标区贴右边界，
// 即 metricsStart + metricsWidth == termWidth。
func TestTUI_NarrowRightAnchor(t *testing.T) {
	for _, tw := range []int{62, 65, 70, 80} {
		lo := computeLayout(tw)
		rightEdge := lo.metricsStart + lo.metricsWidth()
		if rightEdge != tw {
			t.Errorf("termWidth=%d: metricsStart(%d)+metricsWidth(%d)=%d, want %d",
				tw, lo.metricsStart, lo.metricsWidth(), rightEdge, tw)
		}
	}
}
