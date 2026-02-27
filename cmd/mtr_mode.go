package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nxtrace/NTrace-core/config"
	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/util"
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

// runMTRTUI 执行 MTR 交互式 TUI 模式。
// 当 stdin 为 TTY 时启用全屏 TUI（备用屏幕、按键控制）；
// 非 TTY 时降级为简单表格刷新。
func runMTRTUI(method trace.Method, conf trace.Config, intervalMs int, maxRounds int, domain string, dataOrigin string, showIPs bool) {
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

	// 解析源 IP：--source > --dev 推导 > udp dial fallback
	srcHost, _ := os.Hostname()
	srcIP := resolveSrcIP(conf)

	// 语言：默认为 "cn"
	lang := conf.Lang
	if lang == "" {
		lang = "cn"
	}

	// preferred API 信息（仅 LeoMoeAPI 且有结果时展示）
	apiInfo := buildAPIInfo(dataOrigin)

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
			srcHost, srcIP, lang, apiInfo, showIPs, ui.IsPaused, ui.CurrentDisplayMode, ui.CurrentNameMode)
	} else {
		onSnapshot = func(iteration int, stats []trace.MTRHopStat) {
			printer.MTRTablePrinter(stats, iteration, ui.CurrentDisplayMode(), ui.CurrentNameMode(), lang, showIPs)
		}
	}

	err := trace.RunMTR(ctx, method, conf, opts, onSnapshot)
	if err != nil && err != context.Canceled {
		// 离开备用屏幕后再打印错误
		ui.Leave()
		fmt.Println(err)
	}
}

// runMTRReport 执行 MTR 非全屏报告模式（对齐 mtr -rzw 风格）。
// 探测完 maxRounds 后一次性输出最终统计到 stdout，不进入 alternate screen。
func runMTRReport(method trace.Method, conf trace.Config, intervalMs int, maxRounds int, domain string, dataOrigin string, wide bool, showIPs bool) {
	if intervalMs <= 0 {
		intervalMs = 1000
	}
	if maxRounds <= 0 {
		maxRounds = 10
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	startTime := time.Now()

	srcHost, _ := os.Hostname()
	if srcHost == "" {
		srcHost = "unknown-host"
	}

	lang := conf.Lang
	if lang == "" {
		lang = "cn"
	}

	// 最终快照
	var finalStats []trace.MTRHopStat
	onSnapshot := func(iteration int, stats []trace.MTRHopStat) {
		finalStats = stats
	}

	opts := trace.MTROptions{
		Interval:  time.Duration(intervalMs) * time.Millisecond,
		MaxRounds: maxRounds,
	}

	err := trace.RunMTR(ctx, method, conf, opts, onSnapshot)
	if err != nil && err != context.Canceled {
		fmt.Println(err)
		return
	}

	if len(finalStats) == 0 {
		fmt.Println("No data collected.")
		return
	}

	printer.MTRReportPrint(finalStats, printer.MTRReportOptions{
		StartTime: startTime,
		SrcHost:   srcHost,
		Wide:      wide,
		ShowIPs:   showIPs,
		Lang:      lang,
	})
}

// resolveSrcIP 按优先级解析源 IP：--source > --dev 推导 > udp dial fallback。
// 保证与目标 IP 族匹配，失败时返回 "unknown"。
func resolveSrcIP(conf trace.Config) string {
	// 1. --source 已指定
	if conf.SrcAddr != "" {
		return conf.SrcAddr
	}

	// 2. --dev 推导（已在 cmd.go 中赋值到 conf.SrcAddr，这里做兜底）
	if util.SrcDev != "" {
		if dev, err := net.InterfaceByName(util.SrcDev); err == nil {
			if addrs, err2 := dev.Addrs(); err2 == nil {
				for _, addr := range addrs {
					if ipNet, ok := addr.(*net.IPNet); ok {
						if (ipNet.IP.To4() == nil) == (conf.DstIP.To4() == nil) {
							return ipNet.IP.String()
						}
					}
				}
			}
		}
	}

	// 3. udp dial fallback
	if conf.DstIP != nil {
		if c, err := net.Dial("udp", net.JoinHostPort(conf.DstIP.String(), "80")); err == nil {
			if addr, ok := c.LocalAddr().(*net.UDPAddr); ok {
				c.Close()
				return addr.IP.String()
			}
			c.Close()
		}
	}

	return "unknown"
}

// buildAPIInfo 生成首行 preferred API 扩展信息（纯文本，不含 ANSI；仅 LeoMoeAPI）。
func buildAPIInfo(dataOrigin string) string {
	if !strings.EqualFold(dataOrigin, "LeoMoeAPI") && !strings.EqualFold(dataOrigin, "leomoeapi") {
		return ""
	}
	meta := util.FastIPMetaCache
	if meta.IP == "" {
		return ""
	}
	nodeName := meta.NodeName
	if nodeName == "" {
		nodeName = "Unknown"
	}
	return fmt.Sprintf("preferred API IP: %s[%s]", nodeName, meta.IP)
}
