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
	displayMode int32       // 显示模式 0-4（atomic）
	nameMode    int32       // Host 基础显示 0=PTR/IP, 1=IP only（atomic）
	cancel      context.CancelFunc
}

// newMTRUI 创建 TUI 控制器。cancel 是用于退出 MTR 的 context cancel 函数。
// initialDisplayMode 设置 TUI 初始显示模式 (0-4)。
// stdin 和 stdout 都必须是终端才会启用交互式 TUI。
func newMTRUI(cancel context.CancelFunc, initialDisplayMode int) *mtrUI {
	return &mtrUI{
		isTTY:       term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())),
		cancel:      cancel,
		displayMode: int32(initialDisplayMode),
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

// CycleDisplayMode 循环切换显示模式 (0 → 1 → 2 → 3 → 4 → 0)。
func (u *mtrUI) CycleDisplayMode() {
	for {
		old := atomic.LoadInt32(&u.displayMode)
		next := (old + 1) % 5
		if atomic.CompareAndSwapInt32(&u.displayMode, old, next) {
			return
		}
	}
}

// CurrentDisplayMode 返回当前显示模式 (0-4)。
func (u *mtrUI) CurrentDisplayMode() int {
	return int(atomic.LoadInt32(&u.displayMode))
}

// ToggleNameMode 在 PTR/IP (0) 和 IP only (1) 之间切换。
func (u *mtrUI) ToggleNameMode() int32 {
	for {
		old := atomic.LoadInt32(&u.nameMode)
		next := int32(1) - old // 0→1, 1→0
		if atomic.CompareAndSwapInt32(&u.nameMode, old, next) {
			return next
		}
	}
}

// CurrentNameMode 返回当前 Host 基础显示模式 (0=PTR/IP, 1=IP only)。
func (u *mtrUI) CurrentNameMode() int {
	return int(atomic.LoadInt32(&u.nameMode))
}

// ---------------------------------------------------------------------------
// 终端模式关闭序列（幂等）
// ---------------------------------------------------------------------------

// disableTerminalInputModes 向 stdout 写入显式关闭序列，
// 确保鼠标事件、焦点事件、bracketed paste 等不会污染 MTR 输入。
// 在 Enter() 和 Leave() 中均调用，实现幂等防御。
func disableTerminalInputModes() {
	seqs := []string{
		"\033[?1000l", // 关闭 X10 mouse
		"\033[?1002l", // 关闭 button-event mouse
		"\033[?1003l", // 关闭 any-event mouse
		"\033[?1006l", // 关闭 SGR extended mouse
		"\033[?1015l", // 关闭 urxvt mouse
		"\033[?1004l", // 关闭 focus in/out
		"\033[?2004l", // 关闭 bracketed paste
	}
	for _, s := range seqs {
		os.Stdout.WriteString(s)
	}
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
	// 防御：关闭可能被外部残留的鼠标/焦点/paste 模式
	disableTerminalInputModes()
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
	// 防御：确保退出前关闭所有扩展输入模式
	disableTerminalInputModes()
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

// ---------------------------------------------------------------------------
// 输入解析器：字节流状态机
// ---------------------------------------------------------------------------

// mtrInputAction 表示输入解析器解析出的动作。
type mtrInputAction int

const (
	mtrActionNone        mtrInputAction = iota // 无动作（序列被吞掉或 buffer 不足）
	mtrActionQuit                              // q / Q / Ctrl-C
	mtrActionPause                             // p
	mtrActionResume                            // 空格
	mtrActionRestart                           // r
	mtrActionDisplayMode                       // y
	mtrActionNameToggle                        // n
)

// mtrInputParser 是一个字节级状态机，能区分普通按键与
// CSI/SS3/OSC/鼠标/焦点等转义序列，对后者整体吞掉。
type mtrInputParser struct {
	state mtrParserState
	csiN  int // CSI 体内已读字节数（用于限制吞掉长度）
}

type mtrParserState int

const (
	mtrStateGround   mtrParserState = iota // 等待新输入
	mtrStateEsc                            // 刚收到 ESC (0x1B)
	mtrStateCSI                            // ESC [  ——  CSI 序列体
	mtrStateSS3                            // ESC O  ——  SS3 序列体（1 字节负载）
	mtrStateOSC                            // ESC ]  ——  OSC 序列体（到 BEL/ST 结束）
	mtrStateX10Mouse                       // ESC [ M  —— 3 字节负载
	mtrStateSGRMouse                       // ESC [ <  —— 到 M/m 结束
)

// mtrParserMaxCSI 限制 CSI 序列体最大长度，防止畸形输入卡死解析器。
const mtrParserMaxCSI = 64

// Feed 向解析器喂入一个字节，返回识别出的动作。
// 正常按键立即返回动作；转义序列在完整吞掉前返回 mtrActionNone。
func (p *mtrInputParser) Feed(b byte) mtrInputAction {
	switch p.state {
	case mtrStateGround:
		if b == 0x1B { // ESC
			p.state = mtrStateEsc
			return mtrActionNone
		}
		return mapKeyToAction(b)

	case mtrStateEsc:
		switch b {
		case '[':
			p.state = mtrStateCSI
			p.csiN = 0
			return mtrActionNone
		case 'O':
			p.state = mtrStateSS3
			return mtrActionNone
		case ']':
			p.state = mtrStateOSC
			return mtrActionNone
		default:
			// ESC + 非序列头 → 忽略两个字节，回到 ground
			p.state = mtrStateGround
			return mtrActionNone
		}

	case mtrStateCSI:
		p.csiN++
		switch {
		case b == 'M': // X10 mouse: ESC [ M Cb Cx Cy
			p.state = mtrStateX10Mouse
			p.csiN = 0
			return mtrActionNone
		case b == '<': // SGR mouse: ESC [ < params M/m
			p.state = mtrStateSGRMouse
			return mtrActionNone
		case b == 'I' || b == 'O': // Focus in/out: ESC [ I / ESC [ O
			p.state = mtrStateGround
			return mtrActionNone
		case b >= 0x40 && b <= 0x7E: // CSI 终止符
			p.state = mtrStateGround
			return mtrActionNone
		case p.csiN > mtrParserMaxCSI: // 安全限制
			p.state = mtrStateGround
			return mtrActionNone
		default: // CSI 参数/中间字节，继续吞
			return mtrActionNone
		}

	case mtrStateSS3:
		// SS3 序列只有 1 字节最终字符
		p.state = mtrStateGround
		return mtrActionNone

	case mtrStateOSC:
		// OSC 到 BEL (0x07) 或 ST (ESC \) 结束
		if b == 0x07 {
			p.state = mtrStateGround
		} else if b == 0x1B {
			// 可能是 ST 的 ESC 前缀；简化处理：回到 mtrStateEsc
			p.state = mtrStateEsc
		}
		return mtrActionNone

	case mtrStateX10Mouse:
		p.csiN++
		if p.csiN >= 3 { // 吃完 Cb Cx Cy 3 字节
			p.state = mtrStateGround
		}
		return mtrActionNone

	case mtrStateSGRMouse:
		// SGR mouse: 数字、分号、最终 M 或 m
		if b == 'M' || b == 'm' {
			p.state = mtrStateGround
		}
		return mtrActionNone

	default:
		p.state = mtrStateGround
		return mtrActionNone
	}
}

// mapKeyToAction 将普通单字节映射为动作。
func mapKeyToAction(b byte) mtrInputAction {
	switch b {
	case 'q', 'Q', 0x03: // q / Q / Ctrl-C
		return mtrActionQuit
	case 'p', 'P':
		return mtrActionPause
	case ' ':
		return mtrActionResume
	case 'r', 'R':
		return mtrActionRestart
	case 'y', 'Y':
		return mtrActionDisplayMode
	case 'n', 'N':
		return mtrActionNameToggle
	default:
		return mtrActionNone
	}
}

// ReadKeysLoop 在独立 goroutine 中读按键：
//
//	q / Q → 退出（调用 cancel）
//	p     → 暂停
//	空格  → 恢复
//	r     → 重置统计
//	y     → 切换显示模式
//	n     → 切换 Host 显示
//
// 使用 mtrInputParser 字节流状态机解析输入，
// 自动吞掉 CSI/SS3/OSC/鼠标/焦点等转义序列。
// 当 ctx 结束或 stdin 关闭时自动退出。非 TTY 时立即返回。
func (u *mtrUI) ReadKeysLoop(ctx context.Context) {
	if !u.isTTY {
		return
	}
	var parser mtrInputParser
	buf := make([]byte, 64) // 批量读取，减少 syscall
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
		for i := 0; i < n; i++ {
			action := parser.Feed(buf[i])
			switch action {
			case mtrActionQuit:
				u.cancel()
				return
			case mtrActionPause:
				atomic.StoreInt32(&u.paused, 1)
			case mtrActionResume:
				atomic.StoreInt32(&u.paused, 0)
			case mtrActionRestart:
				atomic.StoreInt32(&u.restartReq, 1)
			case mtrActionDisplayMode:
				u.CycleDisplayMode()
			case mtrActionNameToggle:
				u.ToggleNameMode()
			}
		}
	}
}

// ConsumeRestartRequest 原子读取并清除重置请求标志。
// 返回 true 表示请求了重置统计。
func (u *mtrUI) ConsumeRestartRequest() bool {
	return atomic.SwapInt32(&u.restartReq, 0) == 1
}

// ParseMTRKey 将单字节解析为操作名称（用于测试）。
// 返回值: "quit", "pause", "resume", "restart", "display_mode", "name_toggle", "" (未知)。
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
	case 'n', 'N':
		return "name_toggle"
	default:
		return ""
	}
}
