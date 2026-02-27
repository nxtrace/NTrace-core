package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nxtrace/NTrace-core/config"
	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/trace"
)

// MTR 模式下与其他输出/功能标志互斥的检查。
// 返回 true 表示存在冲突。
func checkMTRConflicts(flags map[string]bool) (conflict string, ok bool) {
	conflicts := []struct {
		name string
		set  bool
	}{
		{"--table", flags["table"]},
		{"--raw", flags["raw"]},
		{"--classic", flags["classic"]},
		{"--json", flags["json"]},
		{"--report", flags["report"]},
		{"--output", flags["output"]},
		{"--route-path", flags["routePath"]},
		{"--from", flags["from"]},
		{"--fast-trace", flags["fastTrace"]},
		{"--file", flags["file"]},
		{"--deploy", flags["deploy"]},
	}
	for _, c := range conflicts {
		if c.set {
			return c.name, false
		}
	}
	return "", true
}

// runMTRMode 执行 MTR 连续探测模式。
// 当 stdin 为 TTY 时启用交互式 TUI（备用屏幕、按键控制）；
// 非 TTY 时降级为简单表格刷新。
func runMTRMode(method trace.Method, conf trace.Config, intervalMs int, maxRounds int, domain string) {
	if intervalMs <= 0 {
		intervalMs = 1000
	}

	// Ctrl-C 优雅退出
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// 初始化 TUI 控制器
	ui := newMTRUI(cancel)
	ui.Enter()
	defer ui.Leave()

	// 按键读取协程（非 TTY 时内部 no-op）
	go ui.ReadKeysLoop(ctx)

	startTime := time.Now()
	target := conf.DstIP.String()

	// 解析源主机名和源 IP
	srcHost, _ := os.Hostname()
	srcIP := ""
	if conf.DstIP != nil {
		if c, err := net.Dial("udp", net.JoinHostPort(conf.DstIP.String(), "80")); err == nil {
			if addr, ok := c.LocalAddr().(*net.UDPAddr); ok {
				srcIP = addr.IP.String()
			}
			c.Close()
		}
	}

	// 语言：默认为 "cn"
	lang := conf.Lang
	if lang == "" {
		lang = "cn"
	}

	opts := trace.MTROptions{
		Interval:         time.Duration(intervalMs) * time.Millisecond,
		MaxRounds:        maxRounds,
		IsResetRequested: ui.ConsumeRestartRequest,
	}

	// TTY 模式下使用 TUI 渲染器 + 暂停支持，非 TTY 使用简单表格
	var onSnapshot trace.MTROnSnapshot
	if ui.IsTTY() {
		opts.IsPaused = ui.IsPaused
		onSnapshot = printer.MTRTUIPrinter(target, domain, target, config.Version, startTime,
			srcHost, srcIP, lang, ui.IsPaused, ui.CurrentDisplayMode)
	} else {
		onSnapshot = func(iteration int, stats []trace.MTRHopStat) {
			printer.MTRTablePrinter(stats, iteration, ui.CurrentDisplayMode(), lang)
		}
	}

	err := trace.RunMTR(ctx, method, conf, opts, onSnapshot)
	if err != nil && err != context.Canceled {
		// 离开备用屏幕后再打印错误
		ui.Leave()
		fmt.Println(err)
	}
}
