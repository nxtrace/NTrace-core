package cmd

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
)

func TestCheckMTRConflicts_NoConflict(t *testing.T) {
	flags := map[string]bool{
		"table": false, "raw": false, "classic": false,
		"json": false, "report": false, "output": false,
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
		"json": false, "report": false, "output": false,
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
		"json": true, "report": false, "output": false,
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

func TestCheckMTRConflicts_Report(t *testing.T) {
	flags := map[string]bool{
		"table": false, "raw": false, "classic": false,
		"json": false, "report": true, "output": false,
		"routePath": false, "from": false, "fastTrace": false,
		"file": false, "deploy": false,
	}
	conflict, ok := checkMTRConflicts(flags)
	if ok {
		t.Fatal("expected conflict with --report")
	}
	if conflict != "--report" {
		t.Errorf("conflict = %q, want --report", conflict)
	}
}

func TestCheckMTRConflicts_FastTrace(t *testing.T) {
	flags := map[string]bool{
		"table": false, "raw": false, "classic": false,
		"json": false, "report": false, "output": false,
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
		"json": false, "report": false, "output": false,
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
		"json": false, "report": false, "output": false,
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
		"json": true, "report": true, "output": true,
		"routePath": true, "from": true, "fastTrace": true,
		"file": true, "deploy": true,
	}
	_, ok := checkMTRConflicts(flags)
	if ok {
		t.Fatal("expected conflict when all flags are set")
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
