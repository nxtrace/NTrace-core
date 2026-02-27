package cmd

import (
	"context"
	"io"
	"os"
	"sync/atomic"

	"golang.org/x/term"
)

// ---------------------------------------------------------------------------
// MTR 交互式 TUI 控制器
// ---------------------------------------------------------------------------

// mtrUI 管理终端交互状态：备份屏幕、raw mode、按键处理。
type mtrUI struct {
	isTTY       bool
	oldState    *term.State // raw mode 之前的终端状态
	paused      int32       // 0=running, 1=paused（atomic）
	restartReq  int32       // 1=请求重置统计（atomic）
	displayMode int32       // 显示模式 0-3（atomic）
	cancel      context.CancelFunc
}

// newMTRUI 创建 TUI 控制器。cancel 是用于退出 MTR 的 context cancel 函数。
// stdin 和 stdout 都必须是终端才会启用交互式 TUI。
func newMTRUI(cancel context.CancelFunc) *mtrUI {
	return &mtrUI{
		isTTY:  term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())),
		cancel: cancel,
	}
}

// IsTTY 返回 stdin 和 stdout 是否都是终端。
func (u *mtrUI) IsTTY() bool {
	return u.isTTY
}

// CheckTTY 检查给定的 fd 是否都是终端（可测试）。
func CheckTTY(fds ...int) bool {
	for _, fd := range fds {
		if !term.IsTerminal(fd) {
			return false
		}
	}
	return true
}

// IsPaused 返回当前是否处于暂停状态（供 MTROptions.IsPaused 使用）。
func (u *mtrUI) IsPaused() bool {
	return atomic.LoadInt32(&u.paused) == 1
}

// CycleDisplayMode 循环切换显示模式 (0 → 1 → 2 → 3 → 0)。
func (u *mtrUI) CycleDisplayMode() {
	for {
		old := atomic.LoadInt32(&u.displayMode)
		next := (old + 1) % 4
		if atomic.CompareAndSwapInt32(&u.displayMode, old, next) {
			return
		}
	}
}

// CurrentDisplayMode 返回当前显示模式 (0-3)。
func (u *mtrUI) CurrentDisplayMode() int {
	return int(atomic.LoadInt32(&u.displayMode))
}

// Enter 进入交互模式：切换到备用屏幕缓冲区、隐藏光标、开启 raw mode。
// 非 TTY 时为 no-op。
func (u *mtrUI) Enter() {
	if !u.isTTY {
		return
	}
	// 备用屏幕缓冲区
	os.Stdout.WriteString("\033[?1049h")
	// 隐藏光标
	os.Stdout.WriteString("\033[?25l")
	// raw mode
	if oldState, err := term.MakeRaw(int(os.Stdin.Fd())); err == nil {
		u.oldState = oldState
	}
}

// Leave 离开交互模式：恢复终端状态、显示光标、离开备用屏幕。
// 非 TTY / 未 Enter 时为 no-op。必须在 defer 中调用。
func (u *mtrUI) Leave() {
	if !u.isTTY {
		return
	}
	// 恢复终端
	if u.oldState != nil {
		_ = term.Restore(int(os.Stdin.Fd()), u.oldState)
		u.oldState = nil
	}
	// 显示光标
	os.Stdout.WriteString("\033[?25h")
	// 离开备用屏幕缓冲区（恢复之前内容）
	os.Stdout.WriteString("\033[?1049l")
}

// ReadKeysLoop 在独立 goroutine 中读按键：
//
//	q / Q → 退出（调用 cancel）
//	p     → 暂停
//	空格  → 恢复
//
// 当 ctx 结束或 stdin 关闭时自动退出。非 TTY 时立即返回。
func (u *mtrUI) ReadKeysLoop(ctx context.Context) {
	if !u.isTTY {
		return
	}
	buf := make([]byte, 1)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			if err == io.EOF {
				return
			}
			return
		}
		switch buf[0] {
		case 'q', 'Q', 0x03: // q / Q / Ctrl-C
			u.cancel()
			return
		case 'p', 'P':
			atomic.StoreInt32(&u.paused, 1)
		case ' ':
			atomic.StoreInt32(&u.paused, 0)
		case 'r', 'R':
			atomic.StoreInt32(&u.restartReq, 1)
		case 'y', 'Y':
			u.CycleDisplayMode()
		}
	}
}

// ConsumeRestartRequest 原子读取并清除重置请求标志。
// 返回 true 表示请求了重置统计。
func (u *mtrUI) ConsumeRestartRequest() bool {
	return atomic.SwapInt32(&u.restartReq, 0) == 1
}

// ParseMTRKey 将单字节解析为操作名称（用于测试）。
// 返回值: "quit", "pause", "resume", "restart", "" (未知)。
func ParseMTRKey(b byte) string {
	switch b {
	case 'q', 'Q', 0x03:
		return "quit"
	case 'p', 'P':
		return "pause"
	case ' ':
		return "resume"
	case 'r', 'R':
		return "restart"
	case 'y', 'Y':
		return "display_mode"
	default:
		return ""
	}
}
