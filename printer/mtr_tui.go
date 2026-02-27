package printer

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/nxtrace/NTrace-core/trace"
)

// ---------------------------------------------------------------------------
// MTR TUI 全屏帧渲染器
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
}

// tuiLine 在 raw mode 下输出一行并以 \r\n 结束，
// 确保光标回到行首——裸 \n 在 raw mode 下只向下移动不回列。
func tuiLine(b *strings.Builder, format string, a ...any) {
	fmt.Fprintf(b, format, a...)
	b.WriteString("\r\n")
}

// MTRTUIRender 将 MTR TUI 帧渲染到 w。
// 先组帧到 buffer 再一次性写出，所有换行均使用 \r\n 以兼容 raw mode。
func MTRTUIRender(w io.Writer, header MTRTUIHeader, stats []trace.MTRHopStat) {
	var b strings.Builder

	// 清屏（cursor home + erase）
	b.WriteString("\033[H\033[2J")

	statusStr := "Running"
	if header.Status == MTRTUIPaused {
		statusStr = "Paused"
	}

	tuiLine(&b, "nexttrace --mtr %s          %s",
		header.Target, header.StartTime.Format("2006-01-02T15:04:05"))
	tuiLine(&b, "Keys: q - quit   p - pause   SPACE - resume          [%s] Round: %d",
		statusStr, header.Iteration)
	b.WriteString("\r\n") // 空行

	// 列标题
	tuiLine(&b, "%-6s %-40s  %6s  %4s  %8s  %8s  %8s  %8s  %8s",
		"", "Host", "Loss%", "Snt", "Last", "Avg", "Best", "Wrst", "StDev")

	// hop 行
	prevTTL := 0
	for _, s := range stats {
		hopPrefix := formatTUIHopPrefix(s.TTL, prevTTL)
		prevTTL = s.TTL

		host := formatMTRHost(s)

		tuiLine(&b, "%-6s %-40s  %6s  %4d  %8s  %8s  %8s  %8s  %8s",
			hopPrefix,
			truncateStr(host, 40),
			formatLoss(s.Loss),
			s.Snt,
			formatMs(s.Last),
			formatMs(s.Avg),
			formatMs(s.Best),
			formatMs(s.Wrst),
			formatMs(s.StDev),
		)
	}

	fmt.Fprint(w, b.String())
}

// MTRTUIRenderString 将 MTR TUI 帧渲染为字符串（方便测试）。
func MTRTUIRenderString(header MTRTUIHeader, stats []trace.MTRHopStat) string {
	var sb strings.Builder
	MTRTUIRender(&sb, header, stats)
	return sb.String()
}

// formatTUIHopPrefix 返回 mtr 风格的跳数前缀：
//
//	"1.|--"  新 TTL
//	"  |  "  同 TTL 多路径续行
func formatTUIHopPrefix(ttl, prevTTL int) string {
	if ttl == prevTTL {
		return "  |  "
	}
	return fmt.Sprintf("%d.|--", ttl)
}

// truncateStr 截断字符串到 maxLen，超出时添加省略号。
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
func MTRTUIPrinter(target string, startTime time.Time, isPaused func() bool) func(iteration int, stats []trace.MTRHopStat) {
	return func(iteration int, stats []trace.MTRHopStat) {
		status := MTRTUIRunning
		if isPaused != nil && isPaused() {
			status = MTRTUIPaused
		}
		MTRTUIRender(os.Stdout, MTRTUIHeader{
			Target:    target,
			StartTime: startTime,
			Status:    status,
			Iteration: iteration,
		}, stats)
	}
}
