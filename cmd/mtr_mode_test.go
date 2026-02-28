package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/util"
)

func TestCheckMTRConflicts_NoConflict(t *testing.T) {
	flags := map[string]bool{
		"table": false, "raw": false, "classic": false,
		"json": false, "output": false,
		"routePath": false, "from": false, "fastTrace": false,
		"file": false, "deploy": false,
	}
	conflict, ok := checkMTRConflicts(flags)
	if !ok {
		t.Errorf("expected no conflict, got %q", conflict)
	}
}

func TestCheckMTRConflicts_Table(t *testing.T) {
	flags := map[string]bool{
		"table": true, "raw": false, "classic": false,
		"json": false, "output": false,
		"routePath": false, "from": false, "fastTrace": false,
		"file": false, "deploy": false,
	}
	conflict, ok := checkMTRConflicts(flags)
	if ok {
		t.Fatal("expected conflict with --table")
	}
	if conflict != "--table" {
		t.Errorf("conflict = %q, want --table", conflict)
	}
}

func TestCheckMTRConflicts_JSON(t *testing.T) {
	flags := map[string]bool{
		"table": false, "raw": false, "classic": false,
		"json": true, "output": false,
		"routePath": false, "from": false, "fastTrace": false,
		"file": false, "deploy": false,
	}
	conflict, ok := checkMTRConflicts(flags)
	if ok {
		t.Fatal("expected conflict with --json")
	}
	if conflict != "--json" {
		t.Errorf("conflict = %q, want --json", conflict)
	}
}

func TestCheckMTRConflicts_FastTrace(t *testing.T) {
	flags := map[string]bool{
		"table": false, "raw": false, "classic": false,
		"json": false, "output": false,
		"routePath": false, "from": false, "fastTrace": true,
		"file": false, "deploy": false,
	}
	conflict, ok := checkMTRConflicts(flags)
	if ok {
		t.Fatal("expected conflict with --fast-trace")
	}
	if conflict != "--fast-trace" {
		t.Errorf("conflict = %q, want --fast-trace", conflict)
	}
}

func TestCheckMTRConflicts_Deploy(t *testing.T) {
	flags := map[string]bool{
		"table": false, "raw": false, "classic": false,
		"json": false, "output": false,
		"routePath": false, "from": false, "fastTrace": false,
		"file": false, "deploy": true,
	}
	conflict, ok := checkMTRConflicts(flags)
	if ok {
		t.Fatal("expected conflict with --deploy")
	}
	if conflict != "--deploy" {
		t.Errorf("conflict = %q, want --deploy", conflict)
	}
}

func TestCheckMTRConflicts_From(t *testing.T) {
	flags := map[string]bool{
		"table": false, "raw": false, "classic": false,
		"json": false, "output": false,
		"routePath": false, "from": true, "fastTrace": false,
		"file": false, "deploy": false,
	}
	conflict, ok := checkMTRConflicts(flags)
	if ok {
		t.Fatal("expected conflict with --from")
	}
	if conflict != "--from" {
		t.Errorf("conflict = %q, want --from", conflict)
	}
}

func TestCheckMTRConflicts_AllConflicts(t *testing.T) {
	// 多个冲突标志同时设置时，应返回第一个匹配的
	flags := map[string]bool{
		"table": true, "raw": true, "classic": true,
		"json": true, "output": true,
		"routePath": true, "from": true, "fastTrace": true,
		"file": true, "deploy": true,
	}
	_, ok := checkMTRConflicts(flags)
	if ok {
		t.Fatal("expected conflict when all flags are set")
	}
}

func TestCheckMTRConflicts_RawAllowed(t *testing.T) {
	flags := map[string]bool{
		"table": false, "raw": true, "classic": false,
		"json": false, "output": false,
		"routePath": false, "from": false, "fastTrace": false,
		"file": false, "deploy": false,
	}
	if conflict, ok := checkMTRConflicts(flags); !ok {
		t.Fatalf("raw should be allowed in MTR mode, got conflict=%q", conflict)
	}
}

func TestChooseMTRRunMode_RawPriority(t *testing.T) {
	if mode := chooseMTRRunMode(true, true); mode != mtrRunRaw {
		t.Fatalf("raw should take precedence over report, got mode=%v", mode)
	}
	if mode := chooseMTRRunMode(false, true); mode != mtrRunReport {
		t.Fatalf("report mode mismatch, got mode=%v", mode)
	}
	if mode := chooseMTRRunMode(false, false); mode != mtrRunTUI {
		t.Fatalf("tui mode mismatch, got mode=%v", mode)
	}
}

func TestDeriveMTRRoundParams_DefaultsAndOverrides(t *testing.T) {
	tests := []struct {
		name            string
		effectiveReport bool
		queriesExplicit bool
		numMeasurements int
		ttlTimeExplicit bool
		ttlInterval     int
		wantRounds      int
		wantInterval    int
	}{
		{
			name:            "report default rounds",
			effectiveReport: true,
			queriesExplicit: false,
			numMeasurements: 3,
			ttlTimeExplicit: false,
			ttlInterval:     50,
			wantRounds:      10,
			wantInterval:    1000,
		},
		{
			name:            "report explicit q",
			effectiveReport: true,
			queriesExplicit: true,
			numMeasurements: 7,
			ttlTimeExplicit: true,
			ttlInterval:     250,
			wantRounds:      7,
			wantInterval:    250,
		},
		{
			name:            "tui default infinite",
			effectiveReport: false,
			queriesExplicit: false,
			numMeasurements: 9,
			ttlTimeExplicit: false,
			ttlInterval:     10,
			wantRounds:      0,
			wantInterval:    1000,
		},
		{
			name:            "tui explicit q",
			effectiveReport: false,
			queriesExplicit: true,
			numMeasurements: 4,
			ttlTimeExplicit: true,
			ttlInterval:     1200,
			wantRounds:      4,
			wantInterval:    1200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRounds, gotInterval := deriveMTRRoundParams(
				tt.effectiveReport,
				tt.queriesExplicit,
				tt.numMeasurements,
				tt.ttlTimeExplicit,
				tt.ttlInterval,
			)
			if gotRounds != tt.wantRounds || gotInterval != tt.wantInterval {
				t.Fatalf("got rounds=%d interval=%d, want rounds=%d interval=%d",
					gotRounds, gotInterval, tt.wantRounds, tt.wantInterval)
			}
		})
	}
}

func TestDeriveMTRProbeParams_DefaultsAndOverrides(t *testing.T) {
	tests := []struct {
		name                string
		effectiveReport     bool
		queriesExplicit     bool
		numMeasurements     int
		mtrIntervalExplicit bool
		mtrIntervalMs       int
		ttlTimeExplicit     bool
		ttlInterval         int
		wantMaxPerHop       int
		wantHopIntervalMs   int
	}{
		{
			name:              "report default",
			effectiveReport:   true,
			wantMaxPerHop:     10,
			wantHopIntervalMs: 1000,
		},
		{
			name:              "tui default (unlimited)",
			effectiveReport:   false,
			wantMaxPerHop:     0,
			wantHopIntervalMs: 1000,
		},
		{
			name:              "report explicit q",
			effectiveReport:   true,
			queriesExplicit:   true,
			numMeasurements:   20,
			wantMaxPerHop:     20,
			wantHopIntervalMs: 1000,
		},
		{
			name:                "explicit --mtr-interval overrides -i",
			effectiveReport:     true,
			mtrIntervalExplicit: true,
			mtrIntervalMs:       500,
			ttlTimeExplicit:     true,
			ttlInterval:         2000,
			wantMaxPerHop:       10,
			wantHopIntervalMs:   500,
		},
		{
			name:              "explicit -i as compat alias",
			effectiveReport:   true,
			ttlTimeExplicit:   true,
			ttlInterval:       2000,
			wantMaxPerHop:     10,
			wantHopIntervalMs: 2000,
		},
		{
			name:              "tui explicit q",
			effectiveReport:   false,
			queriesExplicit:   true,
			numMeasurements:   5,
			wantMaxPerHop:     5,
			wantHopIntervalMs: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMaxPerHop, gotHopIntervalMs := deriveMTRProbeParams(
				tt.effectiveReport,
				tt.queriesExplicit,
				tt.numMeasurements,
				tt.mtrIntervalExplicit,
				tt.mtrIntervalMs,
				tt.ttlTimeExplicit,
				tt.ttlInterval,
			)
			if gotMaxPerHop != tt.wantMaxPerHop || gotHopIntervalMs != tt.wantHopIntervalMs {
				t.Fatalf("got maxPerHop=%d hopIntervalMs=%d, want maxPerHop=%d hopIntervalMs=%d",
					gotMaxPerHop, gotHopIntervalMs, tt.wantMaxPerHop, tt.wantHopIntervalMs)
			}
		})
	}
}

func TestNormalizeMTRTraceConfig_UsesMTRInternalTTLInterval50(t *testing.T) {
	original := trace.Config{
		TTLInterval:    1200,
		PacketInterval: 25,
		Timeout:        3,
		MaxHops:        18,
		BeginHop:       4,
	}

	normalized := normalizeMTRTraceConfig(original)

	if normalized.TTLInterval != defaultMTRInternalTTLIntervalMs {
		t.Fatalf("normalized TTLInterval = %d, want %d", normalized.TTLInterval, defaultMTRInternalTTLIntervalMs)
	}
	if normalized.PacketInterval != original.PacketInterval {
		t.Fatalf("normalized PacketInterval = %d, want %d", normalized.PacketInterval, original.PacketInterval)
	}
	if normalized.Timeout != original.Timeout || normalized.MaxHops != original.MaxHops || normalized.BeginHop != original.BeginHop {
		t.Fatalf("unexpected mutation of other fields: %+v", normalized)
	}
	if original.TTLInterval != 1200 {
		t.Fatalf("original config was modified in place: %+v", original)
	}
}

func TestDefaultConstants_NormalVsMTR(t *testing.T) {
	if defaultPacketIntervalMs != 50 {
		t.Fatalf("defaultPacketIntervalMs = %d, want 50", defaultPacketIntervalMs)
	}
	if defaultTracerouteTTLIntervalMs != 300 {
		t.Fatalf("defaultTracerouteTTLIntervalMs = %d, want 300", defaultTracerouteTTLIntervalMs)
	}
	if defaultMTRInternalTTLIntervalMs != 50 {
		t.Fatalf("defaultMTRInternalTTLIntervalMs = %d, want 50", defaultMTRInternalTTLIntervalMs)
	}
}

func TestNormalizeMTRReportConfig_NonWideDisablesGeoAndKeepsRDNS(t *testing.T) {
	geoSource := func(_ string, _ time.Duration, _ string, _ bool) (*ipgeo.IPGeoData, error) {
		return &ipgeo.IPGeoData{}, nil
	}
	original := trace.Config{
		TTLInterval:    1200,
		IPGeoSource:    geoSource,
		RDNS:           true,
		AlwaysWaitRDNS: false,
		PacketInterval: 25,
		Timeout:        3,
		MaxHops:        18,
	}

	normalized := normalizeMTRReportConfig(original, false)

	if normalized.TTLInterval != defaultMTRInternalTTLIntervalMs {
		t.Fatalf("normalized TTLInterval = %d, want %d", normalized.TTLInterval, defaultMTRInternalTTLIntervalMs)
	}
	if normalized.IPGeoSource != nil {
		t.Fatal("non-wide report should disable IPGeoSource")
	}
	if !normalized.RDNS {
		t.Fatal("non-wide report should preserve RDNS=true")
	}
	if !normalized.AlwaysWaitRDNS {
		t.Fatal("non-wide report should force AlwaysWaitRDNS when RDNS is enabled")
	}
	if normalized.PacketInterval != original.PacketInterval || normalized.Timeout != original.Timeout || normalized.MaxHops != original.MaxHops {
		t.Fatalf("unexpected mutation of other fields: %+v", normalized)
	}
	if original.IPGeoSource == nil || original.TTLInterval != 1200 || original.AlwaysWaitRDNS {
		t.Fatalf("original config was modified in place: %+v", original)
	}
}

func TestNormalizeMTRReportConfig_NonWideRespectsNoRDNS(t *testing.T) {
	geoSource := func(_ string, _ time.Duration, _ string, _ bool) (*ipgeo.IPGeoData, error) {
		return &ipgeo.IPGeoData{}, nil
	}
	original := trace.Config{
		TTLInterval:    1200,
		IPGeoSource:    geoSource,
		RDNS:           false,
		AlwaysWaitRDNS: false,
	}

	normalized := normalizeMTRReportConfig(original, false)

	if normalized.IPGeoSource != nil {
		t.Fatal("non-wide report should disable IPGeoSource")
	}
	if normalized.RDNS {
		t.Fatal("non-wide report should preserve RDNS=false")
	}
	if normalized.AlwaysWaitRDNS {
		t.Fatal("non-wide report should not force AlwaysWaitRDNS when RDNS is disabled")
	}
}

func TestNormalizeMTRReportConfig_WidePreservesGeoSettings(t *testing.T) {
	geoSource := func(_ string, _ time.Duration, _ string, _ bool) (*ipgeo.IPGeoData, error) {
		return &ipgeo.IPGeoData{}, nil
	}
	original := trace.Config{
		TTLInterval:    1200,
		IPGeoSource:    geoSource,
		RDNS:           true,
		AlwaysWaitRDNS: false,
		PacketInterval: 25,
	}

	normalized := normalizeMTRReportConfig(original, true)

	if normalized.TTLInterval != defaultMTRInternalTTLIntervalMs {
		t.Fatalf("normalized TTLInterval = %d, want %d", normalized.TTLInterval, defaultMTRInternalTTLIntervalMs)
	}
	if normalized.IPGeoSource == nil {
		t.Fatal("wide report should preserve IPGeoSource")
	}
	if !normalized.RDNS {
		t.Fatal("wide report should preserve RDNS=true")
	}
	if normalized.AlwaysWaitRDNS != original.AlwaysWaitRDNS {
		t.Fatalf("wide report should preserve AlwaysWaitRDNS, got %v want %v", normalized.AlwaysWaitRDNS, original.AlwaysWaitRDNS)
	}
	if original.IPGeoSource == nil || original.TTLInterval != 1200 {
		t.Fatalf("original config was modified in place: %+v", original)
	}
}

func TestBuildRawAPIInfoLine_LeoMoeAPI(t *testing.T) {
	old := util.FastIPMetaCache
	t.Cleanup(func() {
		util.FastIPMetaCache = old
	})

	util.FastIPMetaCache = util.FastIPMeta{
		IP:       "2403:18c0:1001:462:dd:38ff:fe48:e0c5",
		Latency:  "21.33",
		NodeName: "DMIT.NRT",
	}

	got := buildRawAPIInfoLine("LeoMoeAPI")
	want := "[NextTrace API] preferred API IP - [2403:18c0:1001:462:dd:38ff:fe48:e0c5] - 21.33ms - DMIT.NRT"
	if got != want {
		t.Fatalf("buildRawAPIInfoLine() = %q, want %q", got, want)
	}
}

func TestWriteMTRRawRuntimeError_WritesToProvidedWriter(t *testing.T) {
	var buf bytes.Buffer
	err := errors.New("hop timeout")
	writeMTRRawRuntimeError(&buf, err)
	if got := buf.String(); got != err.Error()+"\n" {
		t.Fatalf("writeMTRRawRuntimeError() wrote %q", got)
	}
}

// ---------------------------------------------------------------------------
// ParseMTRKey 测试
// ---------------------------------------------------------------------------

func TestParseMTRKey_Quit(t *testing.T) {
	for _, b := range []byte{'q', 'Q', 0x03} {
		if got := ParseMTRKey(b); got != "quit" {
			t.Errorf("ParseMTRKey(%q) = %q, want %q", b, got, "quit")
		}
	}
}

func TestParseMTRKey_Pause(t *testing.T) {
	for _, b := range []byte{'p', 'P'} {
		if got := ParseMTRKey(b); got != "pause" {
			t.Errorf("ParseMTRKey(%q) = %q, want %q", b, got, "pause")
		}
	}
}

func TestParseMTRKey_Resume(t *testing.T) {
	if got := ParseMTRKey(' '); got != "resume" {
		t.Errorf("ParseMTRKey(' ') = %q, want %q", got, "resume")
	}
}

func TestParseMTRKey_Unknown(t *testing.T) {
	for _, b := range []byte{'x', 'z', '1', '\n'} {
		if got := ParseMTRKey(b); got != "" {
			t.Errorf("ParseMTRKey(%q) = %q, want empty", b, got)
		}
	}
}

// ---------------------------------------------------------------------------
// r 键重置测试
// ---------------------------------------------------------------------------

func TestParseMTRKey_Restart(t *testing.T) {
	for _, b := range []byte{'r', 'R'} {
		if got := ParseMTRKey(b); got != "restart" {
			t.Errorf("ParseMTRKey(%q) = %q, want %q", b, got, "restart")
		}
	}
}

func TestParseMTRKey_DisplayMode(t *testing.T) {
	for _, b := range []byte{'y', 'Y'} {
		if got := ParseMTRKey(b); got != "display_mode" {
			t.Errorf("ParseMTRKey(%q) = %q, want %q", b, got, "display_mode")
		}
	}
}

func TestParseMTRKey_Unknown_IncludesY(t *testing.T) {
	// y/Y 现在已有映射，不再返回空
	for _, b := range []byte{'x', 'z', '1', '\n'} {
		if got := ParseMTRKey(b); got != "" {
			t.Errorf("ParseMTRKey(%q) = %q, want empty", b, got)
		}
	}
}

func TestMTRUI_ConsumeRestartRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ui := newMTRUI(cancel)

	// 初始状态：无重置请求
	if ui.ConsumeRestartRequest() {
		t.Error("expected no restart request initially")
	}

	// 模拟按下 r 键
	atomic.StoreInt32(&ui.restartReq, 1)

	// 第一次消费应返回 true
	if !ui.ConsumeRestartRequest() {
		t.Error("expected restart request after setting flag")
	}

	// 第二次消费应返回 false（已被消费）
	if ui.ConsumeRestartRequest() {
		t.Error("expected restart request to be consumed")
	}

	_ = ctx // suppress unused
}

// ---------------------------------------------------------------------------
// 显示模式切换测试
// ---------------------------------------------------------------------------

func TestMTRUI_DisplayModeCycle(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	ui := newMTRUI(cancel)

	// 初始模式为 0
	if got := ui.CurrentDisplayMode(); got != 0 {
		t.Errorf("initial display mode = %d, want 0", got)
	}

	// 循环切换 0 → 1 → 2 → 3 → 0
	ui.CycleDisplayMode()
	if got := ui.CurrentDisplayMode(); got != 1 {
		t.Errorf("after 1st cycle: display mode = %d, want 1", got)
	}

	ui.CycleDisplayMode()
	if got := ui.CurrentDisplayMode(); got != 2 {
		t.Errorf("after 2nd cycle: display mode = %d, want 2", got)
	}

	ui.CycleDisplayMode()
	if got := ui.CurrentDisplayMode(); got != 3 {
		t.Errorf("after 3rd cycle: display mode = %d, want 3", got)
	}

	ui.CycleDisplayMode()
	if got := ui.CurrentDisplayMode(); got != 0 {
		t.Errorf("after 4th cycle: display mode = %d, want 0 (wrap)", got)
	}
}

func TestMTRUI_DisplayModeNotResetByRestart(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	ui := newMTRUI(cancel)

	// 设置显示模式为 2
	ui.CycleDisplayMode() // 0 → 1
	ui.CycleDisplayMode() // 1 → 2

	// 模拟重置请求
	atomic.StoreInt32(&ui.restartReq, 1)
	ui.ConsumeRestartRequest()

	// 显示模式不应被重置
	if got := ui.CurrentDisplayMode(); got != 2 {
		t.Errorf("display mode after restart = %d, want 2 (unchanged)", got)
	}
}

// ---------------------------------------------------------------------------
// CheckTTY / TTY 判定测试
// ---------------------------------------------------------------------------

func TestCheckTTY_PipeFd(t *testing.T) {
	// 管道 fd 不是终端，CheckTTY 应返回 false
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	if CheckTTY(int(r.Fd())) {
		t.Error("pipe read-end should not be a TTY")
	}
	if CheckTTY(int(w.Fd())) {
		t.Error("pipe write-end should not be a TTY")
	}
}

func TestCheckTTY_StdoutRedirected(t *testing.T) {
	// 模拟 "stdin 是 TTY, stdout 被重定向" 场景：
	// 两个 fd 中至少一个非终端 → 应返回 false
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	// 即使 stdin fd 碰巧是终端（CI 中通常不是），
	// 只要 stdout fd 是管道就应为 false
	if CheckTTY(int(os.Stdin.Fd()), int(w.Fd())) {
		t.Error("CheckTTY(stdin, pipe) should be false when stdout is redirected")
	}
}

func TestCheckTTY_EmptyFds(t *testing.T) {
	// 空参数 → vacuously true
	if !CheckTTY() {
		t.Error("CheckTTY() with no args should be true")
	}
}

// ---------------------------------------------------------------------------
// n 键 NameMode 切换测试
// ---------------------------------------------------------------------------

func TestParseMTRKey_NameToggle(t *testing.T) {
	for _, b := range []byte{'n', 'N'} {
		if got := ParseMTRKey(b); got != "name_toggle" {
			t.Errorf("ParseMTRKey(%q) = %q, want %q", b, got, "name_toggle")
		}
	}
}

func TestMTRUI_NameModeToggle(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	ui := newMTRUI(cancel)

	// 初始为 0 (PTRorIP)
	if got := ui.CurrentNameMode(); got != 0 {
		t.Errorf("initial name mode = %d, want 0", got)
	}

	// 切换 → 1 (IPOnly)
	ui.ToggleNameMode()
	if got := ui.CurrentNameMode(); got != 1 {
		t.Errorf("after toggle: name mode = %d, want 1", got)
	}

	// 再切换 → 0 (PTRorIP)
	ui.ToggleNameMode()
	if got := ui.CurrentNameMode(); got != 0 {
		t.Errorf("after 2nd toggle: name mode = %d, want 0", got)
	}
}

func TestMTRUI_NameModeNotResetByRestart(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	ui := newMTRUI(cancel)

	// 设置 nameMode 为 1
	ui.ToggleNameMode()
	if got := ui.CurrentNameMode(); got != 1 {
		t.Fatalf("name mode = %d, want 1", got)
	}

	// 模拟重置请求
	atomic.StoreInt32(&ui.restartReq, 1)
	ui.ConsumeRestartRequest()

	// nameMode 不应被重置
	if got := ui.CurrentNameMode(); got != 1 {
		t.Errorf("name mode after restart = %d, want 1 (unchanged)", got)
	}
}

// ---------------------------------------------------------------------------
// mtrInputParser 测试
// ---------------------------------------------------------------------------

// feedAll 向解析器喂入完整字节流，返回所有非 None 动作。
func feedAll(p *mtrInputParser, data []byte) []mtrInputAction {
	var actions []mtrInputAction
	for _, b := range data {
		a := p.Feed(b)
		if a != mtrActionNone {
			actions = append(actions, a)
		}
	}
	return actions
}

func TestMTRInputParser_IgnoresX10MouseSequence(t *testing.T) {
	// X10 mouse: ESC [ M Cb Cx Cy  —— 6 字节
	// 关键：Cb/Cx/Cy 可以是 0x20（空格），不应触发 resume
	var p mtrInputParser
	// 模拟点击事件：button=0(0x20), x=10(0x2A), y=5(0x25)
	seq := []byte{0x1B, '[', 'M', 0x20, 0x2A, 0x25}
	actions := feedAll(&p, seq)
	if len(actions) != 0 {
		t.Errorf("X10 mouse should produce no actions, got %v", actions)
	}

	// 确认解析器回到 ground：后续 'q' 应正常识别
	a := p.Feed('q')
	if a != mtrActionQuit {
		t.Errorf("after X10 mouse, 'q' should produce quit, got %d", a)
	}
}

func TestMTRInputParser_IgnoresSGRMouseSequence(t *testing.T) {
	// SGR mouse: ESC [ < 0;10;5 M  (按下) 或 ...m (释放)
	var p mtrInputParser
	press := []byte{0x1B, '[', '<', '0', ';', '1', '0', ';', '5', 'M'}
	release := []byte{0x1B, '[', '<', '0', ';', '1', '0', ';', '5', 'm'}

	actions := feedAll(&p, press)
	if len(actions) != 0 {
		t.Errorf("SGR mouse press should produce no actions, got %v", actions)
	}
	actions = feedAll(&p, release)
	if len(actions) != 0 {
		t.Errorf("SGR mouse release should produce no actions, got %v", actions)
	}
}

func TestMTRInputParser_IgnoresFocusSequence(t *testing.T) {
	var p mtrInputParser
	// Focus in: ESC [ I
	focusIn := []byte{0x1B, '[', 'I'}
	actions := feedAll(&p, focusIn)
	if len(actions) != 0 {
		t.Errorf("focus-in should produce no actions, got %v", actions)
	}

	// Focus out: ESC [ O
	focusOut := []byte{0x1B, '[', 'O'}
	actions = feedAll(&p, focusOut)
	if len(actions) != 0 {
		t.Errorf("focus-out should produce no actions, got %v", actions)
	}
}

func TestMTRInputParser_RecognizesNormalKeysAfterEscapeNoise(t *testing.T) {
	var p mtrInputParser

	// 先喂入一堆 escape 噪音（X10 mouse + focus + CSI arrow），然后喂正常键
	noise := []byte{
		0x1B, '[', 'M', 0x20, 0x30, 0x30, // X10 mouse
		0x1B, '[', 'I', // focus in
		0x1B, '[', 'A', // CSI arrow up
	}
	noiseActions := feedAll(&p, noise)
	if len(noiseActions) != 0 {
		t.Errorf("noise should produce no actions, got %v", noiseActions)
	}

	// 现在喂入正常快捷键序列
	keys := []byte{'p', ' ', 'r', 'y', 'n', 'q'}
	expected := []mtrInputAction{
		mtrActionPause,
		mtrActionResume,
		mtrActionRestart,
		mtrActionDisplayMode,
		mtrActionNameToggle,
		mtrActionQuit,
	}
	actions := feedAll(&p, keys)
	if len(actions) != len(expected) {
		t.Fatalf("expected %d actions, got %d: %v", len(expected), len(actions), actions)
	}
	for i, want := range expected {
		if actions[i] != want {
			t.Errorf("action[%d] = %d, want %d", i, actions[i], want)
		}
	}
}

func TestMTRInputParser_SS3Ignored(t *testing.T) {
	// SS3 F (PF1 key): ESC O P
	var p mtrInputParser
	seq := []byte{0x1B, 'O', 'P'}
	actions := feedAll(&p, seq)
	if len(actions) != 0 {
		t.Errorf("SS3 sequence should produce no actions, got %v", actions)
	}
}

func TestMTRInputParser_OSCIgnored(t *testing.T) {
	// OSC title: ESC ] 0 ; t i t l e BEL
	var p mtrInputParser
	seq := []byte{0x1B, ']', '0', ';', 't', 'i', 't', 'l', 'e', 0x07}
	actions := feedAll(&p, seq)
	if len(actions) != 0 {
		t.Errorf("OSC sequence should produce no actions, got %v", actions)
	}
}

func TestMTRInputParser_BracketedPasteCSISwallowed(t *testing.T) {
	// 验证 bracketed paste 开始序列 ESC [ 2 0 0 ~ 被解析器吞掉（作为 CSI），
	// 且序列终止后后续字节正常回到 ground 状态处理。
	// 真正的 bracketed paste 防护在 disableTerminalInputModes 已关闭 2004 模式，
	// 这里仅测试 CSI 终止符 '~' 之后 parser 恢复 ground 的行为。
	var p mtrInputParser
	seq := []byte{
		0x1B, '[', '2', '0', '0', '~', // CSI 2 0 0 ~ → 被吞掉
		'h', 'e', 'l', 'l', 'o', ' ', 'q', // 后续字节回到 ground 正常处理
	}
	actions := feedAll(&p, seq)
	// CSI "200~" 终止在 '~'，之后：
	// 'h','e','l','l','o' → 无映射（mtrActionNone）
	// ' ' → resume
	// 'q' → quit
	if len(actions) < 2 {
		t.Errorf("expected at least resume+quit from post-CSI bytes, got %d actions: %v", len(actions), actions)
	}
	// 验证最后两个 action 是 resume 和 quit
	if len(actions) >= 2 {
		got := actions[len(actions)-2:]
		if got[0] != mtrActionResume {
			t.Errorf("second-to-last action: want resume, got %d", got[0])
		}
		if got[1] != mtrActionQuit {
			t.Errorf("last action: want quit, got %d", got[1])
		}
	}
}
