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
