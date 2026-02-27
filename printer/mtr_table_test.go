package printer

import (
	"fmt"
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
	rows := MTRRenderTable(stats, HostModeFull, HostNamePTRorIP, "en")
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
	rows := MTRRenderTable(stats, HostModeFull, HostNamePTRorIP, "en")
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
	rows := MTRRenderTable(stats, HostModeFull, HostNamePTRorIP, "en")
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
	rows := MTRRenderTable(stats, HostModeFull, HostNamePTRorIP, "en")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	// 无 hostname 时只显示 IP + Geo
	want := "AS15169 8.8.8.8 United States"
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
	rows := MTRRenderTable(stats, HostModeFull, HostNamePTRorIP, "en")
	want := "AS13335 one.one.one.one US, Cloudflare"
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
	rows := MTRRenderTable(stats, HostModeFull, HostNamePTRorIP, "en")
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
	rows := MTRRenderTable(stats, HostModeFull, HostNamePTRorIP, "en")
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
		Domain:    "example.com",
		TargetIP:  "1.1.1.1",
		Version:   "v1.0.0",
		SrcHost:   "myhost",
		SrcIP:     "192.168.1.1",
	}
	result := MTRTUIRenderString(header, nil)

	if !strings.Contains(result, "NextTrace [v1.0.0]") {
		t.Error("missing 'NextTrace [v1.0.0]' in header")
	}
	if !strings.Contains(result, "myhost (192.168.1.1) -> example.com (1.1.1.1)") {
		t.Error("missing src->dst route line in header")
	}
	if !strings.Contains(result, "[Running]") {
		t.Error("missing Running status")
	}
	if !strings.Contains(result, "Round: 5") {
		t.Error("missing round number")
	}
	if !strings.Contains(result, "q-quit") {
		t.Error("missing key hints")
	}
	if !strings.Contains(result, "r-reset") {
		t.Error("missing reset key hint")
	}
	if !strings.Contains(result, "y-display") {
		t.Error("missing display mode key hint")
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
		Version:   "v1.0.0",
	}
	result := MTRTUIRenderString(header, nil)

	if !strings.HasPrefix(result, "\033[H\033[2J") {
		t.Error("frame should start with cursor-home + erase-screen")
	}
	// header 首行应含有 NextTrace 并以 \r\n 结束
	idx := strings.Index(result, "NextTrace [")
	if idx < 0 {
		t.Fatal("missing 'NextTrace [' in header")
	}
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

	if !strings.Contains(result, "1.") {
		t.Error("missing 1. hop prefix")
	}
	if !strings.Contains(result, "2.") {
		t.Error("missing 2. hop prefix")
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

	// 第一行 TTL=2 → "2.", 第二行同 TTL → "  "
	if !strings.Contains(result, "2.") {
		t.Error("missing first multipath hop prefix")
	}
	// 续行前缀 "  " 不易在输出行中唯一匹配，跳过特定验证
	if !strings.Contains(result, "3.") {
		t.Error("missing next TTL prefix")
	}
}

func TestFormatTUIHopPrefix(t *testing.T) {
	cases := []struct {
		ttl, prev, prefixW int
		want               string
	}{
		{1, 0, 4, " 1. "},
		{5, 4, 4, " 5. "},
		{3, 3, 4, "    "},
		{10, 9, 4, "10. "},
		{100, 99, 5, "100. "},
		{5, 5, 5, "     "},
		{1, 0, 5, "  1. "},
	}
	for _, c := range cases {
		got := formatTUIHopPrefix(c.ttl, c.prev, c.prefixW)
		if got != c.want {
			t.Errorf("formatTUIHopPrefix(%d, %d, %d) = %q, want %q", c.ttl, c.prev, c.prefixW, got, c.want)
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
	// 找到含 "1." 前缀且包含 hop IP 的数据行
	var hopLine string
	for _, l := range lines {
		trimmed := strings.TrimLeft(l, " ")
		if strings.HasPrefix(trimmed, "1.") && strings.Contains(l, "10.0.0.1") {
			hopLine = l
			break
		}
	}
	if hopLine == "" {
		t.Fatal("missing hop line with 1. prefix")
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
	lo := computeLayout(200, tuiPrefixW)
	if lo.hostW <= tuiHostDefault {
		t.Errorf("wide terminal: hostW=%d, want > %d", lo.hostW, tuiHostDefault)
	}
	if lo.termWidth != 200 {
		t.Errorf("termWidth=%d, want 200", lo.termWidth)
	}
}

// TestTUI_HostShrinksWhenWidthReduced 窄终端(80列)时 Host 列宽应被压缩。
func TestTUI_HostShrinksWhenWidthReduced(t *testing.T) {
	lo := computeLayout(80, tuiPrefixW)
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

	// 验证 hop 行与表头子行不超宽
	lines := strings.Split(result, "\r\n")
	for _, l := range lines {
		// 跳过清屏序列、信息行和空行
		if l == "" || strings.HasPrefix(l, "\033[") ||
			strings.Contains(l, "NextTrace") || strings.Contains(l, "->") || strings.Contains(l, "Keys:") {
			continue
		}
		w := displayWidthWithTabs(l, tuiTabStop)
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
	lo := computeLayout(0, tuiPrefixW)
	if lo.termWidth != tuiDefaultTerm {
		t.Errorf("termWidth=%d, want default %d", lo.termWidth, tuiDefaultTerm)
	}
	if lo.hostW < tuiHostMin {
		t.Errorf("hostW=%d, want >= %d", lo.hostW, tuiHostMin)
	}
}

// TestTUI_TotalWidthInvariant 验证 computeLayout 的核心不变式：
// 对于 termWidth >= 20（绝对下限），totalWidth() == termWidth（右锚定）。
func TestTUI_TotalWidthInvariant(t *testing.T) {
	for _, tw := range []int{20, 23, 25, 30, 40, 50, 60, 61, 80, 120, 200} {
		lo := computeLayout(tw, tuiPrefixW)
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
		lo := computeLayout(tw, tuiPrefixW)
		rightEdge := lo.metricsStart + lo.metricsWidth()
		if rightEdge != tw {
			t.Errorf("termWidth=%d: metricsStart(%d)+metricsWidth(%d)=%d, want %d",
				tw, lo.metricsStart, lo.metricsWidth(), rightEdge, tw)
		}
	}
}

// ---------------------------------------------------------------------------
// MTR TUI Header 测试（版本、域名/IP、r 键提示）
// ---------------------------------------------------------------------------

func TestMTRTUI_HeaderContainsVersion(t *testing.T) {
	header := MTRTUIHeader{
		Target:    "8.8.8.8",
		StartTime: time.Now(),
		Version:   "v1.3.0",
		Iteration: 1,
	}
	out := mtrTUIRenderStringWithWidth(header, nil, 120)
	if !strings.Contains(out, "NextTrace [v1.3.0]") {
		t.Errorf("header should contain 'NextTrace [v1.3.0]', got:\n%s", out)
	}
}

func TestMTRTUI_HeaderContainsDomainAndIP(t *testing.T) {
	header := MTRTUIHeader{
		Target:    "8.8.8.8",
		Domain:    "dns.google",
		TargetIP:  "8.8.8.8",
		StartTime: time.Now(),
		Iteration: 1,
		SrcHost:   "myhost",
		SrcIP:     "192.168.1.1",
	}
	out := mtrTUIRenderStringWithWidth(header, nil, 120)
	if !strings.Contains(out, "dns.google (8.8.8.8)") {
		t.Errorf("header should contain 'dns.google (8.8.8.8)', got:\n%s", out)
	}
	// 应包含 src -> dst 格式
	if !strings.Contains(out, "myhost (192.168.1.1) -> dns.google (8.8.8.8)") {
		t.Errorf("header should contain src -> dst route line, got:\n%s", out)
	}
}

func TestMTRTUI_HeaderContainsResetKey(t *testing.T) {
	header := MTRTUIHeader{
		Target:    "8.8.8.8",
		StartTime: time.Now(),
		Iteration: 1,
	}
	out := mtrTUIRenderStringWithWidth(header, nil, 120)
	if !strings.Contains(out, "r-reset") {
		t.Errorf("header should contain 'r-reset', got:\n%s", out)
	}
}

func TestMTRTUI_HeaderIPOnlyWhenNoDomain(t *testing.T) {
	header := MTRTUIHeader{
		Target:    "1.2.3.4",
		TargetIP:  "1.2.3.4",
		StartTime: time.Now(),
		Iteration: 1,
	}
	out := mtrTUIRenderStringWithWidth(header, nil, 120)
	// 无域名时只显示 IP（不含 "Host:" 前缀）
	if !strings.Contains(out, "1.2.3.4") {
		t.Errorf("header should contain '1.2.3.4' when domain is empty, got:\n%s", out)
	}
	// 不应出现 "Host:" 前缀（新格式使用 src -> dst）
	if strings.Contains(out, "Host:") {
		t.Errorf("new header should not contain 'Host:' prefix, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// formatTUIHopPrefix 新风格测试
// ---------------------------------------------------------------------------

func TestFormatTUIHopPrefix_MinimalStyle(t *testing.T) {
	// 新 TTL 应返回 "%*d. " 格式（prefixW=4: 2 位 TTL）
	got := formatTUIHopPrefix(1, 0, 4)
	if got != " 1. " {
		t.Errorf("formatTUIHopPrefix(1, 0, 4) = %q, want %q", got, " 1. ")
	}

	got = formatTUIHopPrefix(10, 9, 4)
	if got != "10. " {
		t.Errorf("formatTUIHopPrefix(10, 9, 4) = %q, want %q", got, "10. ")
	}

	// 续行应返回 prefixW 个空格
	got = formatTUIHopPrefix(5, 5, 4)
	if got != "    " {
		t.Errorf("formatTUIHopPrefix(5, 5, 4) = %q, want %q", got, "    ")
	}

	// 3 位 TTL，prefixW=5
	got = formatTUIHopPrefix(100, 99, 5)
	if got != "100. " {
		t.Errorf("formatTUIHopPrefix(100, 99, 5) = %q, want %q", got, "100. ")
	}

	got = formatTUIHopPrefix(5, 5, 5)
	if got != "     " {
		t.Errorf("formatTUIHopPrefix(5, 5, 5) = %q, want %q", got, "     ")
	}
}

// ---------------------------------------------------------------------------
// formatMTRHost MPLS 测试
// ---------------------------------------------------------------------------

func TestFormatMTRHost_IncludesMPLS(t *testing.T) {
	// extractMPLS 产出格式为 "[MPLS: Lbl N, TC N, S N, TTL N]"，不应再包裹
	stat := trace.MTRHopStat{
		TTL:  1,
		IP:   "10.0.0.1",
		MPLS: []string{"[MPLS: Lbl 100, TC 0, S 1, TTL 1]", "[MPLS: Lbl 200, TC 0, S 0, TTL 1]"},
	}
	got := formatMTRHost(stat)
	// 不应出现双层包裹 "[MPLS: [MPLS: ..."
	if strings.Contains(got, "[MPLS: [MPLS:") {
		t.Errorf("should not double-wrap MPLS, got: %q", got)
	}
	// 每个标签应直接出现
	if !strings.Contains(got, "[MPLS: Lbl 100") {
		t.Errorf("should contain first MPLS label, got: %q", got)
	}
	if !strings.Contains(got, "[MPLS: Lbl 200") {
		t.Errorf("should contain second MPLS label, got: %q", got)
	}
}

func TestFormatMTRHost_NoMPLS(t *testing.T) {
	stat := trace.MTRHopStat{
		TTL: 1,
		IP:  "10.0.0.1",
	}
	got := formatMTRHost(stat)
	if strings.Contains(got, "MPLS") {
		t.Errorf("formatMTRHost should not contain MPLS when empty, got: %q", got)
	}
}

// ---------------------------------------------------------------------------
// IP 重复展示测试（P2）
// ---------------------------------------------------------------------------

func TestMTRTUI_HeaderIPNoDuplicate(t *testing.T) {
	// 当 Domain == TargetIP 时不应显示 "1.1.1.1 (1.1.1.1)"
	header := MTRTUIHeader{
		Target:    "1.1.1.1",
		Domain:    "1.1.1.1",
		TargetIP:  "1.1.1.1",
		StartTime: time.Now(),
		Iteration: 1,
	}
	out := mtrTUIRenderStringWithWidth(header, nil, 120)
	if strings.Contains(out, "1.1.1.1 (1.1.1.1)") {
		t.Errorf("should not show duplicate IP, got:\n%s", out)
	}
	// 新格式下目标 IP 应单独出现
	if !strings.Contains(out, "1.1.1.1") {
		t.Errorf("should show '1.1.1.1', got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// 显示模式测试
// ---------------------------------------------------------------------------

func TestFormatMTRHostByMode_ASN(t *testing.T) {
	s := trace.MTRHopStat{
		TTL:  1,
		IP:   "1.1.1.1",
		Host: "one.one.one.one",
		Geo: &ipgeo.IPGeoData{
			Asnumber:  "13335",
			CountryEn: "US",
			Owner:     "Cloudflare",
		},
	}
	got := formatMTRHostByMode(s, HostModeASN, HostNamePTRorIP, "en")
	want := "AS13335 one.one.one.one"
	if got != want {
		t.Errorf("HostModeASN: got %q, want %q", got, want)
	}
}

func TestFormatMTRHostByMode_City(t *testing.T) {
	s := trace.MTRHopStat{
		TTL: 1,
		IP:  "1.1.1.1",
		Geo: &ipgeo.IPGeoData{
			Asnumber:  "13335",
			CountryEn: "US",
			ProvEn:    "California",
			CityEn:    "Los Angeles",
		},
	}
	got := formatMTRHostByMode(s, HostModeCity, HostNamePTRorIP, "en")
	want := "AS13335 1.1.1.1 Los Angeles"
	if got != want {
		t.Errorf("HostModeCity: got %q, want %q", got, want)
	}
}

func TestFormatMTRHostByMode_Owner(t *testing.T) {
	s := trace.MTRHopStat{
		TTL: 1,
		IP:  "1.1.1.1",
		Geo: &ipgeo.IPGeoData{
			Asnumber: "13335",
			Owner:    "Cloudflare",
		},
	}
	got := formatMTRHostByMode(s, HostModeOwner, HostNamePTRorIP, "en")
	want := "AS13335 1.1.1.1 Cloudflare"
	if got != want {
		t.Errorf("HostModeOwner: got %q, want %q", got, want)
	}
}

func TestFormatMTRHostByMode_Full(t *testing.T) {
	s := trace.MTRHopStat{
		TTL:  1,
		IP:   "1.1.1.1",
		Host: "one.one.one.one",
		Geo: &ipgeo.IPGeoData{
			Asnumber:  "13335",
			CountryEn: "US",
			Owner:     "Cloudflare",
		},
	}
	got := formatMTRHostByMode(s, HostModeFull, HostNamePTRorIP, "en")
	want := "AS13335 one.one.one.one US, Cloudflare"
	if got != want {
		t.Errorf("HostModeFull: got %q, want %q", got, want)
	}
}

func TestFormatMTRHostByMode_NilGeo(t *testing.T) {
	s := trace.MTRHopStat{TTL: 1, IP: "10.0.0.1"}
	for _, mode := range []int{HostModeASN, HostModeCity, HostModeOwner, HostModeFull} {
		got := formatMTRHostByMode(s, mode, HostNamePTRorIP, "en")
		if got != "10.0.0.1" {
			t.Errorf("mode %d with nil geo: got %q, want %q", mode, got, "10.0.0.1")
		}
	}
}

func TestFormatMTRHostByMode_NoASN(t *testing.T) {
	s := trace.MTRHopStat{
		TTL: 1,
		IP:  "10.0.0.1",
		Geo: &ipgeo.IPGeoData{CountryEn: "US"},
	}
	got := formatMTRHostByMode(s, HostModeASN, HostNamePTRorIP, "en")
	// 无 ASN 时只显示 base
	if got != "10.0.0.1" {
		t.Errorf("HostModeASN no ASN: got %q, want %q", got, "10.0.0.1")
	}
}

// ---------------------------------------------------------------------------
// 语言感知测试
// ---------------------------------------------------------------------------

func TestGeoField_CN(t *testing.T) {
	got := geoField("中国", "China", "cn")
	if got != "中国" {
		t.Errorf("geoField cn: got %q, want %q", got, "中国")
	}
}

func TestGeoField_EN(t *testing.T) {
	got := geoField("中国", "China", "en")
	if got != "China" {
		t.Errorf("geoField en: got %q, want %q", got, "China")
	}
}

func TestGeoField_Fallback(t *testing.T) {
	// en 模式但无英文字段时回退到中文
	got := geoField("中国", "", "en")
	if got != "中国" {
		t.Errorf("geoField en fallback: got %q, want %q", got, "中国")
	}
	// cn 模式但无中文字段时回退到英文
	got = geoField("", "China", "cn")
	if got != "China" {
		t.Errorf("geoField cn fallback: got %q, want %q", got, "China")
	}
}

func TestFormatMTRHostByMode_LangCN(t *testing.T) {
	s := trace.MTRHopStat{
		TTL: 1,
		IP:  "1.1.1.1",
		Geo: &ipgeo.IPGeoData{
			Asnumber:  "13335",
			Country:   "美国",
			CountryEn: "US",
			Prov:      "加利福尼亚",
			ProvEn:    "California",
			City:      "洛杉矶",
			CityEn:    "Los Angeles",
		},
	}
	got := formatMTRHostByMode(s, HostModeCity, HostNamePTRorIP, "cn")
	want := "AS13335 1.1.1.1 洛杉矶"
	if got != want {
		t.Errorf("HostModeCity cn: got %q, want %q", got, want)
	}

	got = formatMTRHostByMode(s, HostModeCity, HostNamePTRorIP, "en")
	want = "AS13335 1.1.1.1 Los Angeles"
	if got != want {
		t.Errorf("HostModeCity en: got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// MPLS 多行显示测试
// ---------------------------------------------------------------------------

func TestTUI_MPLSMultiLine(t *testing.T) {
	header := MTRTUIHeader{Target: "1.1.1.1", StartTime: time.Now(), Iteration: 1}
	stats := []trace.MTRHopStat{
		{
			TTL:  1,
			IP:   "10.0.0.1",
			Loss: 0, Snt: 5, Last: 1.0, Avg: 1.0, Best: 1.0, Wrst: 1.0, StDev: 0,
			MPLS: []string{"[MPLS: Lbl 100, TC 0, S 1, TTL 1]", "[MPLS: Lbl 200, TC 0, S 0, TTL 1]"},
		},
		{TTL: 2, IP: "10.0.0.2", Loss: 0, Snt: 5, Last: 2.0, Avg: 2.0, Best: 2.0, Wrst: 2.0, StDev: 0},
	}
	result := mtrTUIRenderStringWithWidth(header, stats, 120)

	// MPLS 标签应在独立的续行中出现
	if !strings.Contains(result, "[MPLS: Lbl 100") {
		t.Error("missing first MPLS label in output")
	}
	if !strings.Contains(result, "[MPLS: Lbl 200") {
		t.Error("missing second MPLS label in output")
	}

	// MPLS 不应和 host IP 在同一行
	lines := strings.Split(result, "\r\n")
	for _, l := range lines {
		if strings.Contains(l, "10.0.0.1") && strings.Contains(l, "MPLS") {
			t.Error("MPLS label should not be on the same line as host IP")
		}
	}
}

func TestTUI_MPLSMultiLine_NoMPLS(t *testing.T) {
	header := MTRTUIHeader{Target: "1.1.1.1", StartTime: time.Now(), Iteration: 1}
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "10.0.0.1", Loss: 0, Snt: 5, Last: 1.0, Avg: 1.0, Best: 1.0, Wrst: 1.0, StDev: 0},
	}
	result := mtrTUIRenderStringWithWidth(header, stats, 120)
	if strings.Contains(result, "MPLS") {
		t.Error("output should not contain MPLS when hop has no labels")
	}
}

// ---------------------------------------------------------------------------
// 新 header 格式综合测试
// ---------------------------------------------------------------------------

func TestMTRTUI_HeaderSrcDstFormat(t *testing.T) {
	header := MTRTUIHeader{
		Target:    "8.8.8.8",
		Domain:    "dns.google",
		TargetIP:  "8.8.8.8",
		StartTime: time.Now(),
		Iteration: 1,
		Version:   "v1.0.0",
		SrcHost:   "laptop.local",
		SrcIP:     "10.0.0.5",
	}
	out := mtrTUIRenderStringWithWidth(header, nil, 150)

	// 第一行应仅含 NextTrace [版本]
	lines := strings.Split(out, "\r\n")
	found := false
	for _, l := range lines {
		if strings.Contains(l, "NextTrace [v1.0.0]") {
			found = true
			// 不应含 "My traceroute"
			if strings.Contains(l, "My traceroute") {
				t.Error("line 1 should not contain 'My traceroute'")
			}
			break
		}
	}
	if !found {
		t.Error("missing 'NextTrace [v1.0.0]' in output")
	}

	// 第二行应含 src -> dst + RFC3339 时间
	if !strings.Contains(out, "laptop.local (10.0.0.5) -> dns.google (8.8.8.8)") {
		t.Errorf("missing route line, got:\n%s", out)
	}
}

func TestMTRTUI_HeaderNoSrcInfo(t *testing.T) {
	// 无源信息时应只显示目标
	header := MTRTUIHeader{
		Target:    "8.8.8.8",
		Domain:    "dns.google",
		TargetIP:  "8.8.8.8",
		StartTime: time.Now(),
		Iteration: 1,
		Version:   "v1.0.0",
	}
	out := mtrTUIRenderStringWithWidth(header, nil, 120)
	// 不应含 "->" （无源信息）
	if strings.Contains(out, "->") {
		t.Errorf("should not contain '->' when no src info, got:\n%s", out)
	}
	// 应含目标
	if !strings.Contains(out, "dns.google (8.8.8.8)") {
		t.Errorf("should contain destination, got:\n%s", out)
	}
}

func TestMTRTUI_DisplayModeInKeys(t *testing.T) {
	for mode, label := range map[int]string{0: "ASN", 1: "City", 2: "Owner", 3: "Full"} {
		header := MTRTUIHeader{
			Target:      "8.8.8.8",
			StartTime:   time.Now(),
			Iteration:   1,
			DisplayMode: mode,
		}
		out := mtrTUIRenderStringWithWidth(header, nil, 120)
		expected := "y-display(" + label + ")"
		if !strings.Contains(out, expected) {
			t.Errorf("mode %d: should contain %q, got:\n%s", mode, expected, out)
		}
	}
}

func TestMTRTUI_DisplayModeAffectsHopData(t *testing.T) {
	stats := []trace.MTRHopStat{
		{
			TTL: 1, IP: "1.1.1.1",
			Geo: &ipgeo.IPGeoData{
				Asnumber:  "13335",
				CountryEn: "US",
				Owner:     "Cloudflare",
			},
			Loss: 0, Snt: 5, Last: 1.0, Avg: 1.0, Best: 1.0, Wrst: 1.0, StDev: 0,
		},
	}

	// Mode 0 (ASN): 有 ASN 无 country 无 owner
	header := MTRTUIHeader{Target: "x", StartTime: time.Now(), Iteration: 1, DisplayMode: HostModeASN, Lang: "en"}
	out := mtrTUIRenderStringWithWidth(header, stats, 120)
	if !strings.Contains(out, "AS13335") {
		t.Error("mode ASN should show ASN")
	}
	if strings.Contains(out, "Cloudflare") {
		t.Error("mode ASN should not show owner")
	}

	// Mode 3 (Full): 有 ASN + country + owner
	header.DisplayMode = HostModeFull
	out = mtrTUIRenderStringWithWidth(header, stats, 120)
	if !strings.Contains(out, "AS13335") {
		t.Error("mode Full should show ASN")
	}
	if !strings.Contains(out, "US") {
		t.Error("mode Full should show country")
	}
	if !strings.Contains(out, "Cloudflare") {
		t.Error("mode Full should show owner")
	}
}

// ---------------------------------------------------------------------------
// NameMode (n 键) 测试
// ---------------------------------------------------------------------------

func TestFormatMTRHostByMode_IPOnly_ShowsIP(t *testing.T) {
	s := trace.MTRHopStat{
		TTL:  1,
		IP:   "1.1.1.1",
		Host: "one.one.one.one",
		Geo: &ipgeo.IPGeoData{
			Asnumber:  "13335",
			CountryEn: "US",
			Owner:     "Cloudflare",
		},
	}
	// HostNameIPOnly 时 base 始终是 IP，即使有 PTR
	got := formatMTRHostByMode(s, HostModeFull, HostNameIPOnly, "en")
	want := "AS13335 1.1.1.1 US, Cloudflare"
	if got != want {
		t.Errorf("HostModeFull+IPOnly: got %q, want %q", got, want)
	}
}

func TestFormatMTRHostByMode_PTRorIP_ShowsPTR(t *testing.T) {
	s := trace.MTRHopStat{
		TTL:  1,
		IP:   "1.1.1.1",
		Host: "one.one.one.one",
		Geo: &ipgeo.IPGeoData{
			Asnumber:  "13335",
			CountryEn: "US",
			Owner:     "Cloudflare",
		},
	}
	got := formatMTRHostByMode(s, HostModeFull, HostNamePTRorIP, "en")
	want := "AS13335 one.one.one.one US, Cloudflare"
	if got != want {
		t.Errorf("HostModeFull+PTRorIP: got %q, want %q", got, want)
	}
}

func TestFormatMTRHostByMode_IPOnly_NoPTR(t *testing.T) {
	// 无 PTR 时 HostNameIPOnly 和 HostNamePTRorIP 结果相同
	s := trace.MTRHopStat{
		TTL: 1,
		IP:  "10.0.0.1",
		Geo: &ipgeo.IPGeoData{Asnumber: "64512"},
	}
	gotIP := formatMTRHostByMode(s, HostModeASN, HostNameIPOnly, "en")
	gotPTR := formatMTRHostByMode(s, HostModeASN, HostNamePTRorIP, "en")
	if gotIP != gotPTR {
		t.Errorf("no PTR: IPOnly=%q differs from PTRorIP=%q", gotIP, gotPTR)
	}
}

func TestMTRRenderTable_IPOnly(t *testing.T) {
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "1.1.1.1", Host: "one.one.one.one", Geo: &ipgeo.IPGeoData{
			Asnumber:  "13335",
			CountryEn: "US",
		}},
	}
	// HostNameIPOnly → Host 使用 IP 而非 PTR
	rows := MTRRenderTable(stats, HostModeFull, HostNameIPOnly, "en")
	if strings.Contains(rows[0].Host, "one.one.one.one") {
		t.Errorf("IPOnly should not show PTR, got: %q", rows[0].Host)
	}
	if !strings.Contains(rows[0].Host, "1.1.1.1") {
		t.Errorf("IPOnly should show IP, got: %q", rows[0].Host)
	}
}

// ---------------------------------------------------------------------------
// TUI Header APIInfo + NameMode 测试
// ---------------------------------------------------------------------------

func TestMTRTUI_HeaderAPIInfo(t *testing.T) {
	header := MTRTUIHeader{
		Target:    "8.8.8.8",
		StartTime: time.Now(),
		Iteration: 1,
		Version:   "v1.0.0",
		APIInfo:   "preferred API IP: 1.2.3.4",
	}
	out := mtrTUIRenderStringWithWidth(header, nil, 120)
	if !strings.Contains(out, "preferred API IP: 1.2.3.4") {
		t.Errorf("header should contain API info, got:\n%s", out)
	}
	// API 信息应与 NextTrace 在同一行
	lines := strings.Split(out, "\r\n")
	for _, l := range lines {
		if strings.Contains(l, "NextTrace [v1.0.0]") {
			if !strings.Contains(l, "preferred API IP: 1.2.3.4") {
				t.Errorf("API info should be on the same line as NextTrace, got:\n%s", l)
			}
			return
		}
	}
	t.Error("missing NextTrace header line")
}

func TestMTRTUI_HeaderNoAPIInfo(t *testing.T) {
	header := MTRTUIHeader{
		Target:    "8.8.8.8",
		StartTime: time.Now(),
		Iteration: 1,
		Version:   "v1.0.0",
	}
	out := mtrTUIRenderStringWithWidth(header, nil, 120)
	if strings.Contains(out, "preferred API") {
		t.Errorf("header should not contain API info when empty, got:\n%s", out)
	}
}

func TestMTRTUI_NameModeInKeys(t *testing.T) {
	for nm, label := range map[int]string{0: "ptr", 1: "ip"} {
		header := MTRTUIHeader{
			Target:    "8.8.8.8",
			StartTime: time.Now(),
			Iteration: 1,
			NameMode:  nm,
		}
		out := mtrTUIRenderStringWithWidth(header, nil, 120)
		expected := "n-host(" + label + ")"
		if !strings.Contains(out, expected) {
			t.Errorf("nameMode %d: should contain %q, got:\n%s", nm, expected, out)
		}
	}
}

func TestMTRTUI_FirstLineCentered(t *testing.T) {
	header := MTRTUIHeader{
		Target:    "8.8.8.8",
		StartTime: time.Now(),
		Iteration: 1,
		Version:   "v1.0.0",
	}
	out := mtrTUIRenderStringWithWidth(header, nil, 120)
	lines := strings.Split(out, "\r\n")
	for _, l := range lines {
		if strings.Contains(l, "NextTrace [v1.0.0]") {
			trimmed := strings.TrimRight(l, " ")
			// 居中意味着首字符不是 'N'
			if len(trimmed) > 0 && trimmed[0] == 'N' {
				t.Errorf("first line should be centered (left-padded), got: %q", l)
			}
			return
		}
	}
	t.Error("missing NextTrace header line")
}

// TestMTRTUI_FirstLineTruncatedOnNarrow 验证首行在极窄终端不会超宽。
func TestMTRTUI_FirstLineTruncatedOnNarrow(t *testing.T) {
	header := MTRTUIHeader{
		Target:    "8.8.8.8",
		StartTime: time.Now(),
		Iteration: 1,
		Version:   "v1.0.0",
		APIInfo:   "preferred API IP: 123.456.789.012 some very long extra text to overflow",
	}
	const width = 30
	out := mtrTUIRenderStringWithWidth(header, nil, width)
	lines := strings.Split(out, "\r\n")
	for _, l := range lines {
		// 跳过清屏序列开头的行：去掉 ANSI CSI 序列后再检查
		clean := l
		for strings.HasPrefix(clean, "\033[") {
			// 跳过 \033[ ... 直到终止字节 (0x40-0x7E)
			idx := 2
			for idx < len(clean) && (clean[idx] < 0x40 || clean[idx] > 0x7E) {
				idx++
			}
			if idx < len(clean) {
				idx++ // 跳过终止字节
			}
			clean = clean[idx:]
		}
		if strings.Contains(clean, "NextTrace") {
			w := displayWidth(clean)
			if w > width {
				t.Errorf("first line exceeds termWidth=%d: displayWidth=%d, line=%q", width, w, clean)
			}
			return
		}
	}
	t.Error("missing NextTrace header line")
}

// ---------------------------------------------------------------------------
// 新增：waiting for reply + 分隔符规则测试
// ---------------------------------------------------------------------------

// TestTUI_WaitingForReplyOn100Loss 验证 100% loss 且无地址的 hop 在 TUI 中
// 显示 "(waiting for reply)" 而非 "???"。
func TestTUI_WaitingForReplyOn100Loss(t *testing.T) {
	header := MTRTUIHeader{Target: "1.1.1.1", StartTime: time.Now(), Iteration: 1}
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "10.0.0.1", Loss: 0, Snt: 5, Last: 1.0, Avg: 1.0, Best: 1.0, Wrst: 1.0, StDev: 0},
		{TTL: 2, IP: "", Host: "", Loss: 100, Snt: 5, Last: 0, Avg: 0, Best: 0, Wrst: 0, StDev: 0},
		{TTL: 3, IP: "10.0.0.3", Loss: 0, Snt: 5, Last: 2.0, Avg: 2.0, Best: 2.0, Wrst: 2.0, StDev: 0},
	}
	result := mtrTUIRenderStringWithWidth(header, stats, 120)

	if !strings.Contains(result, "(waiting for reply)") {
		t.Errorf("100%% loss hop should show '(waiting for reply)', got:\n%s", result)
	}
	// 不应出现 "???"（TUI 中的 100% loss 无地址 hop）
	lines := strings.Split(result, "\r\n")
	for _, l := range lines {
		// 排除 header/表头行，只看数据行
		trimmed := strings.TrimLeft(l, " ")
		if strings.HasPrefix(trimmed, "2.") && strings.Contains(l, "???") {
			t.Errorf("100%% loss hop should not show '???' in TUI, got line: %q", l)
		}
	}
}

// TestTUI_HostSeparators_WithTabs 验证 TUI 中 host 文本的 tab 分隔规则：
//   - 序号后 1 空格 + ASN
//   - ASN 与 IP/PTR 之间为 tab
//   - IP/PTR 与后续信息之间为 tab
func TestTUI_HostSeparators_WithTabs(t *testing.T) {
	header := MTRTUIHeader{
		Target:      "1.1.1.1",
		StartTime:   time.Now(),
		Iteration:   1,
		DisplayMode: HostModeFull,
		Lang:        "en",
	}
	stats := []trace.MTRHopStat{
		{
			TTL: 1, IP: "1.1.1.1", Host: "one.one.one.one",
			Loss: 0, Snt: 5, Last: 1.0, Avg: 1.0, Best: 1.0, Wrst: 1.0, StDev: 0,
			Geo: &ipgeo.IPGeoData{
				Asnumber:  "13335",
				CountryEn: "US",
				Owner:     "Cloudflare",
			},
		},
	}
	result := mtrTUIRenderStringWithWidth(header, stats, 120)

	lines := strings.Split(result, "\r\n")
	var hopLine string
	for _, l := range lines {
		if strings.Contains(l, "one.one.one.one") {
			hopLine = l
			break
		}
	}
	if hopLine == "" {
		t.Fatal("missing hop line with one.one.one.one")
	}

	// 序号后 1 空格：应含 " 1. AS" 模式
	if !strings.Contains(hopLine, " 1. AS") {
		t.Errorf("prefix should be followed by 1 space then ASN, got: %q", hopLine)
	}

	// ASN 与 IP/PTR 之间应有 tab
	if !strings.Contains(hopLine, "AS13335\tone.one.one.one") {
		t.Errorf("ASN and IP/PTR should be separated by tab, got: %q", hopLine)
	}

	// IP/PTR 与后续信息之间应有 tab
	if !strings.Contains(hopLine, "one.one.one.one\tUS Cloudflare") {
		t.Errorf("IP/PTR and extras should be separated by tab, got: %q", hopLine)
	}
}

// TestTUI_TabAwareAlignment_StillRightAnchored 验证含 tab 的 host 行
// 右侧指标区仍能对齐（metricsStart 稳定）。
func TestTUI_TabAwareAlignment_StillRightAnchored(t *testing.T) {
	header := MTRTUIHeader{
		Target:      "1.1.1.1",
		StartTime:   time.Now(),
		Iteration:   1,
		DisplayMode: HostModeASN,
		Lang:        "en",
	}
	stats := []trace.MTRHopStat{
		{
			TTL: 1, IP: "10.0.0.1", Loss: 0, Snt: 5, Last: 1.0, Avg: 1.0, Best: 1.0, Wrst: 1.0, StDev: 0,
			Geo: &ipgeo.IPGeoData{Asnumber: "13335"},
		},
		{
			TTL: 2, IP: "10.0.0.2", Loss: 50, Snt: 4, Last: 5.0, Avg: 6.0, Best: 4.0, Wrst: 8.0, StDev: 1.5,
			Geo: &ipgeo.IPGeoData{Asnumber: "174"},
		},
		{
			TTL: 3, IP: "", Host: "", Loss: 100, Snt: 4, Last: 0, Avg: 0, Best: 0, Wrst: 0, StDev: 0,
		},
	}
	const width = 120
	result := mtrTUIRenderStringWithWidth(header, stats, width)

	lo := computeLayout(width, tuiPrefixWidthForMaxTTL(3))
	lines := strings.Split(result, "\r\n")

	// 在数据行中，指标区应出现在 metricsStart 附近
	for _, l := range lines {
		trimmed := strings.TrimLeft(l, " ")
		if len(trimmed) == 0 {
			continue
		}
		// 检查是否是数据行（以 "N. " 格式开头）
		isData := false
		for _, prefix := range []string{"1.", "2.", "3."} {
			if strings.HasPrefix(trimmed, prefix) {
				isData = true
				break
			}
		}
		if !isData {
			continue
		}
		// 行的 tab-aware 宽度不应超过 termWidth
		w := displayWidthWithTabs(l, tuiTabStop)
		if w > width {
			t.Errorf("data row exceeds termWidth=%d: displayWidth=%d, line=%q", width, w, l)
		}
		// 指标区不应超出
		if w > lo.metricsStart+lo.metricsWidth() {
			t.Errorf("data row overflows: displayWidth=%d > metricsStart(%d)+metricsWidth(%d), line=%q",
				w, lo.metricsStart, lo.metricsWidth(), l)
		}
	}
}

// TestReport_WaitingForReplyOn100Loss 验证 report 模式中 100% loss
// 且无地址的 hop 显示 "(waiting for reply)"。
func TestReport_WaitingForReplyOn100Loss(t *testing.T) {
	p := buildMTRHostParts(trace.MTRHopStat{
		TTL: 2, IP: "", Host: "", Loss: 100, Snt: 10,
	}, HostModeFull, HostNamePTRorIP, "en")
	if !p.waiting {
		t.Fatal("expected waiting=true for 100% loss with no IP/Host")
	}

	host := formatReportHost(trace.MTRHopStat{
		TTL: 2, IP: "", Host: "", Loss: 100, Snt: 10,
	}, HostModeFull, HostNamePTRorIP, "en")
	if host != "(waiting for reply)" {
		t.Errorf("report host = %q, want %q", host, "(waiting for reply)")
	}

	// 有 IP 但 loss=100% → 不应显示 waiting
	hostWithIP := formatReportHost(trace.MTRHopStat{
		TTL: 2, IP: "10.0.0.1", Host: "", Loss: 100, Snt: 10,
	}, HostModeFull, HostNamePTRorIP, "en")
	if hostWithIP == "(waiting for reply)" {
		t.Error("hop with IP should not show waiting even with 100% loss")
	}
}

// TestReport_FullExtrasUseSpaces_NoComma 验证 report HostModeFull 中
// 后续信息使用空格分隔，不含 ", "。
func TestReport_FullExtrasUseSpaces_NoComma(t *testing.T) {
	s := trace.MTRHopStat{
		TTL:  1,
		IP:   "1.1.1.1",
		Host: "one.one.one.one",
		Loss: 0, Snt: 10,
		Geo: &ipgeo.IPGeoData{
			Asnumber:  "13335",
			CountryEn: "US",
			Owner:     "Cloudflare",
		},
	}
	host := formatReportHost(s, HostModeFull, HostNamePTRorIP, "en")
	// 应为 "AS13335 one.one.one.one US Cloudflare"（空格分隔）
	if strings.Contains(host, ", ") {
		t.Errorf("report host should not contain ', ', got: %q", host)
	}
	want := "AS13335 one.one.one.one US Cloudflare"
	if host != want {
		t.Errorf("report host = %q, want %q", host, want)
	}
}

// ---------------------------------------------------------------------------
// 3 位 TTL 前缀宽度回归测试
// ---------------------------------------------------------------------------

// TestTUI_ThreeDigitTTLAlignment 验证 TTL>=100 时前缀宽度自动扩展为 5，
// 布局不变式仍成立，且数据行不超宽。
func TestTUI_ThreeDigitTTLAlignment(t *testing.T) {
	header := MTRTUIHeader{Target: "1.1.1.1", StartTime: time.Now(), Iteration: 1, Lang: "en"}
	var stats []trace.MTRHopStat
	for ttl := 99; ttl <= 101; ttl++ {
		stats = append(stats, trace.MTRHopStat{
			TTL: ttl, IP: fmt.Sprintf("10.0.%d.1", ttl),
			Loss: 0, Snt: 5, Last: 1.0, Avg: 1.0, Best: 1.0, Wrst: 1.0, StDev: 0,
		})
	}

	const width = 120
	result := mtrTUIRenderStringWithWidth(header, stats, width)

	// 验证 prefixW 为 5（3 位 TTL → digits=3 → prefixW=5）
	prefixW := tuiPrefixWidthForMaxTTL(101)
	if prefixW != 5 {
		t.Errorf("tuiPrefixWidthForMaxTTL(101) = %d, want 5", prefixW)
	}

	// 布局不变式
	lo := computeLayout(width, prefixW)
	if lo.totalWidth() != width {
		t.Errorf("totalWidth()=%d, want %d (prefixW=%d, hostW=%d)", lo.totalWidth(), width, lo.prefixW, lo.hostW)
	}

	// 100. 前缀应出现在输出中
	if !strings.Contains(result, "100.") {
		t.Error("missing '100.' prefix in output")
	}
	if !strings.Contains(result, "101.") {
		t.Error("missing '101.' prefix in output")
	}

	// 数据行不超宽
	lines := strings.Split(result, "\r\n")
	for _, l := range lines {
		trimmed := strings.TrimLeft(l, " ")
		if len(trimmed) == 0 {
			continue
		}
		isData := false
		for _, pfx := range []string{"99.", "100.", "101."} {
			if strings.HasPrefix(trimmed, pfx) {
				isData = true
				break
			}
		}
		if !isData {
			continue
		}
		w := displayWidthWithTabs(l, tuiTabStop)
		if w > width {
			t.Errorf("data row exceeds termWidth=%d: displayWidth=%d, line=%q", width, w, l)
		}
	}
}

// TestTUI_PrefixWidthForMaxTTL 验证 tuiPrefixWidthForMaxTTL 各区间。
func TestTUI_PrefixWidthForMaxTTL(t *testing.T) {
	cases := []struct {
		maxTTL int
		want   int
	}{
		{0, 4}, // 空 stats → 默认
		{1, 4}, // TTL<100 → 2 digits + 2 = 4
		{99, 4},
		{100, 5}, // 3 digits + 2 = 5
		{255, 5},
		{999, 5},
		{1000, 6}, // 4 digits + 2 = 6（极端场景）
	}
	for _, c := range cases {
		got := tuiPrefixWidthForMaxTTL(c.maxTTL)
		if got != c.want {
			t.Errorf("tuiPrefixWidthForMaxTTL(%d) = %d, want %d", c.maxTTL, got, c.want)
		}
	}
}

// TestTUI_TotalWidthInvariant_ThreeDigitTTL 验证 3 位 TTL 下右锚定不变式。
func TestTUI_TotalWidthInvariant_ThreeDigitTTL(t *testing.T) {
	prefixW := tuiPrefixWidthForMaxTTL(100) // 5
	for _, tw := range []int{21, 25, 30, 40, 60, 80, 120, 200} {
		lo := computeLayout(tw, prefixW)
		if lo.totalWidth() != tw {
			t.Errorf("termWidth=%d, prefixW=%d: totalWidth()=%d, want exact match",
				tw, prefixW, lo.totalWidth())
		}
		if lo.hostW < 1 {
			t.Errorf("termWidth=%d, prefixW=%d: hostW=%d, must be >= 1", tw, prefixW, lo.hostW)
		}
	}
}

// ---------------------------------------------------------------------------
// Waiting-hop blank metrics
// ---------------------------------------------------------------------------

// TestMTRRenderTable_WaitingMetricsBlank 验证 100% 丢包且无 IP/Host 的行
// 的所有指标列均为空字符串。
func TestMTRRenderTable_WaitingMetricsBlank(t *testing.T) {
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "1.1.1.1", Loss: 0, Snt: 5, Last: 1.0, Avg: 1.0, Best: 1.0, Wrst: 1.0, StDev: 0},
		{TTL: 2, IP: "", Host: "", Loss: 100, Snt: 5}, // waiting
		{TTL: 3, IP: "2.2.2.2", Loss: 10, Snt: 5, Last: 2.0, Avg: 2.0, Best: 2.0, Wrst: 2.0, StDev: 0},
	}
	rows := MTRRenderTable(stats, HostModeFull, HostNamePTRorIP, "en")
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}

	// TTL 1 应有正常指标
	if rows[0].Loss == "" {
		t.Error("TTL 1 Loss should not be empty")
	}

	// TTL 2 应全部空白
	r := rows[1]
	for _, pair := range []struct {
		name, val string
	}{
		{"Loss", r.Loss}, {"Snt", r.Snt}, {"Last", r.Last},
		{"Avg", r.Avg}, {"Best", r.Best}, {"Wrst", r.Wrst}, {"StDev", r.StDev},
	} {
		if pair.val != "" {
			t.Errorf("waiting row %s = %q, want empty", pair.name, pair.val)
		}
	}

	// TTL 3 应有正常指标
	if rows[2].Loss == "" {
		t.Error("TTL 3 Loss should not be empty")
	}
}

// TestTUI_WaitingMetricsBlank 验证 TUI 帧中 waiting 行不出现 "100.0%" 或 "0.00"。
func TestTUI_WaitingMetricsBlank(t *testing.T) {
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "1.1.1.1", Loss: 0, Snt: 5, Last: 1.0, Avg: 1.0, Best: 1.0, Wrst: 1.0, StDev: 0},
		{TTL: 2, IP: "", Host: "", Loss: 100, Snt: 5}, // waiting
	}
	header := MTRTUIHeader{
		SrcHost: "localhost",
		Target:  "example.com",
	}
	frame := mtrTUIRenderStringWithWidth(header, stats, 120)

	// Split lines and find the waiting row (contains "(waiting for reply)")
	lines := strings.Split(frame, "\n")
	var waitingLine string
	for _, l := range lines {
		if strings.Contains(l, "(waiting for reply)") {
			waitingLine = l
			break
		}
	}
	if waitingLine == "" {
		t.Fatal("no waiting-for-reply line found in TUI frame")
	}
	if strings.Contains(waitingLine, "100.0%") {
		t.Errorf("waiting line should not contain '100.0%%': %q", waitingLine)
	}
	if strings.Contains(waitingLine, "0.00") {
		t.Errorf("waiting line should not contain '0.00': %q", waitingLine)
	}
}
