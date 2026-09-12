package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nxtrace/NTrace-core/config"
	"github.com/nxtrace/NTrace-core/internal/mtrsession"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/util"
	"golang.org/x/term"
)

const defaultMTRInternalTTLIntervalMs = 0

// MTR 模式下与其他输出/功能标志互斥的检查。
// 返回 true 表示存在冲突。
func checkMTRConflicts(flags map[string]bool) (conflict string, ok bool) {
	conflicts := []struct {
		name string
		set  bool
	}{
		{"--table", flags["table"]},
		{"--classic", flags["classic"]},
		{"--output", flags["output"]},
		{"--output-default", flags["outputDefault"]},
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
func runMTRTUI(method trace.Method, conf trace.Config, hopIntervalMs int, maxPerHop int, domain string, dataOrigin string, showIPs bool, initialDisplayMode int, onEvent func(trace.MTRSessionEvent) error, options mtrTUIOptions, columns ...printer.MTRColumn) (runErr error) {
	if hopIntervalMs <= 0 {
		hopIntervalMs = 1000
	}

	sigCtx, stop := mtrRunContext(conf)
	defer stop()
	ctx, cancel := context.WithCancel(sigCtx)
	defer cancel()

	// 初始化 TUI 控制器
	ui := newMTRUI(cancel, initialDisplayMode)
	ui.columns = append([]printer.MTRColumn(nil), columns...)

	startTime := time.Now()
	target := conf.DstIP.String()

	// 解析源 IP：--source > --dev 推导 > udp dial fallback
	srcHost, _ := os.Hostname()
	if srcHost == "" {
		srcHost = "unknown-host"
	}
	srcIP := resolveSrcIP(conf)

	// 语言：默认为 "cn"
	lang := conf.Lang
	if lang == "" {
		lang = "cn"
	}

	roundConf := normalizeMTRTraceConfig(conf)
	snapshots := &mtrLiveSnapshots{start: startTime, session: mtrsession.Session{
		Version: config.Version, Target: domain, ResolvedIP: target, Protocol: string(method), StartedAt: startTime.UTC(),
		SourceHost: srcHost, SourceIP: srcIP,
		EffectiveParameters: buildMTRSnapshotParameters(method, roundConf, hopIntervalMs, maxPerHop, dataOrigin, options),
	}}
	if snapshots.session.Target == "" {
		snapshots.session.Target = target
	}
	ui.captureSnapshot = func() *printer.MTRSnapshot { return captureMTRDisplay(&snapshots.store, ui, showIPs) }
	var probeErr error
	ui.Enter()
	defer func() {
		ui.Leave()
		if err := ui.closeSnapshotSaver(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		if ui.IsTTY() && !options.NoSummary {
			summaryErr := probeErr
			if summaryErr == nil {
				summaryErr = context.Cause(ctx)
			}
			if err := writeMTRExitSummary(os.Stdout, ui.captureSnapshot(), summaryErr); err != nil {
				fmt.Fprintf(os.Stderr, "write MTR summary: %v\n", err)
			}
		}
	}()
	keysCtx, stopKeys := context.WithCancel(ctx)
	keysDone := make(chan struct{})
	go func() { defer close(keysDone); ui.ReadKeysLoop(keysCtx) }()
	defer func() { stopKeys(); <-keysDone }()

	opts := buildMTRInteractiveOptions(ui, hopIntervalMs, maxPerHop)
	opts.OnEvent = onEvent
	if ui.IsTTY() {
		opts.OnEvent = func(event trace.MTRSessionEvent) error {
			snapshots.event(event)
			if onEvent != nil {
				return onEvent(event)
			}
			return nil
		}
		opts.OnPathEnd = func(reason *trace.StopReason) { snapshots.pathEnd = copyMTRPathEnd(reason) }
	}
	history := attachMTRHistoryIfTTY(ui, &opts)

	// TTY 模式下使用 TUI 渲染器 + 暂停支持，非 TTY 使用简单表格
	var onSnapshot trace.MTROnSnapshot
	if ui.IsTTY() {
		opts.IsPaused = ui.IsPaused
		var frameColumns []printer.MTRColumn
		var frameEditor printer.MTRColumnEditor
		var frameSave printer.MTRSaveDialog
		var frameHelp printer.MTRHelpDialog
		render := printer.MTRTUIPrinterWithDialogs(target, domain, target, config.Version, startTime,
			srcHost, srcIP, lang, func() string { return buildAPIInfo(dataOrigin) }, showIPs, ui.IsPaused,
			ui.CurrentDisplayMode, ui.CurrentNameMode, ui.IsMPLSDisabled,
			ui.IsHistoryMode, ui.CurrentHistoryChartMode, history.Snapshot, func() ([]printer.MTRColumn, printer.MTRColumnEditor) { return frameColumns, frameEditor },
			func() (printer.MTRSaveDialog, printer.MTRHelpDialog) { return frameSave, frameHelp })
		pasteEnabled := false
		var stopRedraw func()
		onSnapshot, stopRedraw = startMTRRedraw(ui.redraw, func() (int, int) { w, h, _ := term.GetSize(int(os.Stdout.Fd())); return w, h }, func(n int, stats []trace.MTRHopStat) {
			frameColumns, frameEditor = ui.columnSnapshot()
			frameSave, frameHelp = ui.dialogSnapshot()
			editing := frameEditor.Active || frameSave.Active || frameHelp.Active
			if editing != pasteEnabled {
				pasteEnabled = editing
				if pasteEnabled {
					_, _ = fmt.Fprint(os.Stdout, "\033[?2004h")
				} else {
					_, _ = fmt.Fprint(os.Stdout, "\033[?2004l")
				}
			}
			render(n, stats)
		})
		displaySnapshot := onSnapshot
		onSnapshot = func(n int, stats []trace.MTRHopStat) {
			snapshots.snapshot(stats)
			displaySnapshot(n, stats)
		}
		defer stopRedraw()
	} else {
		onSnapshot = func(iteration int, stats []trace.MTRHopStat) {
			printer.MTRTablePrinterWithColumns(stats, iteration, ui.CurrentDisplayMode(), ui.CurrentNameMode(), lang, showIPs, columns)
		}
	}

	probeErr = trace.RunMTR(ctx, method, roundConf, opts, onSnapshot)
	return mtrRunError(ctx, probeErr)
}

func buildMTRInteractiveOptions(ui *mtrUI, hopIntervalMs int, maxPerHop int) trace.MTROptions {
	opts := trace.MTROptions{
		HopInterval: time.Duration(hopIntervalMs) * time.Millisecond,
		MaxPerHop:   maxPerHop,
	}
	if ui == nil {
		return opts
	}
	opts.IsResetRequested = ui.ConsumeRestartRequest
	opts.AsyncMetadata = ui.IsTTY()
	return opts
}

func attachMTRHistoryIfTTY(ui *mtrUI, opts *trace.MTROptions) *printer.MTRHistoryStore {
	if ui == nil || opts == nil || !ui.IsTTY() {
		return nil
	}
	history := printer.NewMTRHistoryStore(printer.MTRHistoryWindow)
	opts.OnProbe = history.AddProbeEvent
	if opts.IsResetRequested != nil {
		resetRequested := opts.IsResetRequested
		opts.IsResetRequested = func() bool {
			if !resetRequested() {
				return false
			}
			history.Reset()
			return true
		}
	}
	return history
}

// runMTRReport 执行 MTR 非全屏报告模式（对齐 mtr -rzw 风格）。
// 探测完 maxPerHop 后一次性输出最终统计到 stdout，不进入 alternate screen。
func runMTRReport(method trace.Method, conf trace.Config, hopIntervalMs int, maxPerHop int, domain string, dataOrigin string, wide bool, showIPs bool, onEvent func(trace.MTRSessionEvent) error, columns ...printer.MTRColumn) error {
	if hopIntervalMs <= 0 {
		hopIntervalMs = 1000
	}
	if maxPerHop <= 0 {
		maxPerHop = 10
	}

	ctx, stop := mtrRunContext(conf)
	defer stop()

	startTime := time.Now()

	srcHost, _ := os.Hostname()
	if srcHost == "" {
		srcHost = "unknown-host"
	}

	lang := conf.Lang
	if lang == "" {
		lang = "cn"
	}

	opts := trace.MTROptions{
		HopInterval: time.Duration(hopIntervalMs) * time.Millisecond,
		MaxPerHop:   maxPerHop,
		OnEvent:     onEvent,
	}

	roundConf := normalizeMTRReportConfig(conf, wide)
	finalStats, _, err := collectMTRReport(ctx, method, roundConf, opts)
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.Cause(ctx)) {
		return err
	}

	if len(finalStats) == 0 {
		fmt.Println("No data collected.")
		return mtrRunError(ctx, err)
	}

	printer.MTRReportPrint(finalStats, printer.MTRReportOptions{
		Columns:   columns,
		StartTime: startTime,
		SrcHost:   srcHost,
		Wide:      wide,
		ShowIPs:   showIPs,
		Lang:      lang,
	})
	return mtrRunError(ctx, err)
}

// runMTRRaw 执行 MTR 原始流式模式（逐事件输出，'|' 分隔）。
// 行格式固定为 12 列：
// ttl|ip|ptr|rtt|asn|country|prov|city|district|owner|lat|lng
func writeMTRRawPathEnd(w io.Writer, reason *trace.StopReason) error {
	if reason == nil || reason.Reason != trace.StopReasonUnreachable {
		return nil
	}
	return printer.WriteTraceStopReason(w, reason)
}

func runMTRRaw(method trace.Method, conf trace.Config, hopIntervalMs int, maxPerHop int, dataOrigin string, onEvent func(trace.MTRSessionEvent) error) error {
	if hopIntervalMs <= 0 {
		hopIntervalMs = 1000
	}

	sigCtx, stop := mtrRunContext(conf)
	defer stop()
	ctx, cancel := context.WithCancelCause(sigCtx)
	defer cancel(nil)

	opts := trace.MTRRawOptions{
		HopInterval: time.Duration(hopIntervalMs) * time.Millisecond,
		MaxPerHop:   maxPerHop,
		OnEvent:     onEvent,
		OnPathEnd: func(reason *trace.StopReason) {
			if err := writeMTRRawPathEnd(os.Stderr, reason); err != nil {
				writeMTRRawRuntimeError(os.Stderr, err)
			}
		},
	}

	roundConf := normalizeMTRTraceConfig(conf)
	if apiLine := buildRawAPIInfoLine(dataOrigin); apiLine != "" {
		if _, err := fmt.Fprintln(os.Stdout, apiLine); err != nil {
			return err
		}
	}

	err := trace.RunMTRRaw(ctx, method, roundConf, opts, func(rec trace.MTRRawRecord) {
		if _, writeErr := fmt.Fprintln(os.Stdout, printer.FormatMTRRawLine(rec)); writeErr != nil {
			cancel(writeErr)
		}
	})
	return mtrRunError(ctx, err)
}

func mtrRunContext(conf trace.Config) (context.Context, context.CancelFunc) {
	// The caller owns signals when it supplies a context. Another signal
	// subscriber could cancel first and lose the caller's interruption cause.
	if conf.Context != nil {
		return conf.Context, func() {}
	}
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func mtrRunError(ctx context.Context, err error) error {
	if cause := context.Cause(ctx); cause != nil && (err == nil || errors.Is(err, context.Canceled)) {
		return cause
	}
	return err
}

func normalizeMTRTraceConfig(conf trace.Config) trace.Config {
	normalized := conf
	normalized.TTLInterval = defaultMTRInternalTTLIntervalMs
	return normalized
}

func normalizeMTRReportConfig(conf trace.Config, wide bool) trace.Config {
	normalized := normalizeMTRTraceConfig(conf)
	if wide {
		return normalized
	}

	normalized.IPGeoSource = nil
	normalized.IPGeoDescriptor = nil
	normalized.RefreshIPGeoSource = nil
	if normalized.RDNS {
		normalized.AlwaysWaitRDNS = true
	}
	return normalized
}

func writeMTRRawRuntimeError(w io.Writer, err error) {
	if err == nil || w == nil {
		return
	}
	_, _ = fmt.Fprintln(w, err)
}

// resolveSrcIP 按优先级解析源 IP：--source > --dev 推导 > udp dial fallback。
// 保证与目标 IP 族匹配，失败时返回 "unknown"。
func resolveSrcIP(conf trace.Config) string {
	sourceDevice := conf.SourceDevice
	if sourceDevice == "" {
		sourceDevice = util.SrcDev
	}
	resolved, _, err := trace.ResolveConfiguredSrcAddr(conf.DstIP, conf.SrcAddr, sourceDevice)
	if err == nil && strings.TrimSpace(resolved) != "" {
		return resolved
	}
	return "unknown"
}

// buildAPIInfo 生成首行 preferred API 扩展信息（纯文本，不含 ANSI；仅 NextTrace API）。
func buildAPIInfo(dataOrigin string) string {
	if !ipgeo.IsNextTraceAPIProvider(dataOrigin) {
		return ""
	}
	meta := util.GetFastIPMetaCache()
	if meta.IP == "" {
		return ""
	}
	nodeName := meta.NodeName
	if nodeName == "" {
		nodeName = "Unknown"
	}
	return fmt.Sprintf("preferred API IP: %s[%s]", nodeName, meta.IP)
}

func buildRawAPIInfoLine(dataOrigin string) string {
	if !ipgeo.IsNextTraceAPIProvider(dataOrigin) {
		return ""
	}
	meta := util.GetFastIPMetaCache()
	if meta.IP == "" {
		return ""
	}

	nodeName := strings.TrimSpace(meta.NodeName)
	if nodeName == "" {
		nodeName = "Unknown"
	}
	latency := strings.TrimSpace(meta.Latency)
	if latency == "" {
		return fmt.Sprintf("[NextTrace API] preferred API IP - [%s] - %s", meta.IP, nodeName)
	}
	return fmt.Sprintf("[NextTrace API] preferred API IP - [%s] - %sms - %s", meta.IP, latency, nodeName)
}
