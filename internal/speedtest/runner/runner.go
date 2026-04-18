package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nxtrace/NTrace-core/internal/speedtest"
	speedconfig "github.com/nxtrace/NTrace-core/internal/speedtest/config"
	"github.com/nxtrace/NTrace-core/internal/speedtest/latency"
	"github.com/nxtrace/NTrace-core/internal/speedtest/netx"
	"github.com/nxtrace/NTrace-core/internal/speedtest/provider"
	"github.com/nxtrace/NTrace-core/internal/speedtest/provider/apple"
	"github.com/nxtrace/NTrace-core/internal/speedtest/provider/cloudflare"
	"github.com/nxtrace/NTrace-core/internal/speedtest/render"
	"github.com/nxtrace/NTrace-core/internal/speedtest/result"
	"github.com/nxtrace/NTrace-core/internal/speedtest/transfer"
	"github.com/nxtrace/NTrace-core/trace"
)

type candidate struct {
	IP     string
	Desc   string
	RTTMs  float64
	Source string
	Status string
	Error  string
	Meta   map[string]any
}

var (
	resolveAllIPsFn   = netx.ResolveAllIPs
	newHTTPClientFn   = netx.NewClient
	openPromptInputFn = openPromptInput
)

func Run(ctx context.Context, cfg *speedconfig.Config, bus *render.Bus, isTTY bool) result.RunResult {
	started := time.Now()
	res := result.RunResult{
		SchemaVersion: 1,
		Config: result.RunConfig{
			Provider:       cfg.Provider,
			Max:            cfg.Max,
			MaxBytes:       cfg.MaxBytes,
			TimeoutMs:      cfg.TimeoutMs,
			Threads:        cfg.Threads,
			LatencyCount:   cfg.LatencyCount,
			JSON:           cfg.OutputJSON,
			NonInteractive: cfg.NonInteractive,
			EndpointIP:     cfg.EndpointIP,
			Metadata:       !cfg.NoMetadata,
			Language:       cfg.Language,
			NoColor:        cfg.NoColor,
			DotServer:      cfg.DotServer,
			Source:         cfg.SourceAddress,
			Device:         cfg.SourceDevice,
		},
		SelectedEndpoint: result.SelectedEndpoint{Status: "unavailable"},
		ConnectionInfo: result.ConnectionInfo{
			Status:          "unavailable",
			MetadataEnabled: !cfg.NoMetadata,
			Client:          result.PeerInfo{Status: "unavailable"},
			Server:          result.PeerInfo{Status: "unavailable"},
		},
		IdleLatency: result.LatencyResult{Status: "unavailable"},
		StartedAt:   started.UTC().Format(time.RFC3339Nano),
	}

	p, err := buildProvider(cfg)
	if err != nil {
		addWarning(&res, "provider", err.Error())
		return finalize(started, res, 1)
	}

	if bus != nil {
		bus.Line()
		bus.Banner("NextTrace Speed")
		bus.Info(speedtest.Text(cfg.Language, "Config:  ", "配置:  ") + cfg.Summary())
		bus.Line()
	}

	discovery, err := discoverCandidates(ctx, cfg, p)
	if err != nil {
		addWarning(&res, "discovery", err.Error())
		return finalize(started, res, 1)
	}
	for _, w := range discovery.Warnings {
		addWarning(&res, w.Code, w.Message)
	}
	res.Candidates = candidateResults(discovery.Candidates)
	res.SelectedEndpoint = selectedEndpoint(discovery.Selected)

	if bus != nil {
		renderSelection(bus, ctx, cfg, &discovery, isTTY && !cfg.NonInteractive && !cfg.OutputJSON)
		res.SelectedEndpoint = selectedEndpoint(discovery.Selected)
	}
	if discovery.Selected.IP == "" {
		addWarning(&res, "selection", speedtest.Text(cfg.Language, "no endpoint selected", "未选择测速节点"))
		return finalize(started, res, 1)
	}
	if interrupted(ctx) {
		return finalize(started, res, 130)
	}

	host := p.Host()
	res.ConnectionInfo = gatherConnectionInfo(ctx, cfg, host, discovery.Selected)
	if !cfg.NoMetadata && res.ConnectionInfo.Status != "ok" {
		res.Degraded = true
	}
	if bus != nil {
		renderConnectionInfo(bus, cfg, res.ConnectionInfo)
	}

	client, err := newPinnedClient(cfg, host, discovery.Selected.IP, time.Duration(cfg.TimeoutMs+5000)*time.Millisecond)
	if err != nil {
		addWarning(&res, "client", err.Error())
		return finalize(started, res, 1)
	}

	idleProbe := func(ctx context.Context) (float64, error) {
		spec, err := p.IdleLatencyRequest(ctx)
		if err != nil {
			return -1, err
		}
		ms, _, err := performRequest(ctx, client, spec, p)
		return ms, err
	}
	if bus != nil {
		bus.Header(speedtest.Text(cfg.Language, "Idle Latency", "空载延迟"))
		bus.Info(fmt.Sprintf(speedtest.Text(cfg.Language, "Samples: %d", "采样: %d"), cfg.LatencyCount))
	}
	idleStats := latency.MeasureIdle(ctx, cfg.LatencyCount, idleProbe)
	res.IdleLatency = latencyResult(idleStats, speedtest.Text(cfg.Language, "No latency samples collected.", "未采集到延迟样本。"))
	if res.IdleLatency.Status != "ok" {
		res.Degraded = true
	}
	if bus != nil {
		renderLatency(bus, cfg, res.IdleLatency)
	}

	runRound := func(dir transfer.Direction, threads int, title string) {
		if interrupted(ctx) {
			return
		}
		if bus != nil {
			bus.Header(title)
			bus.Info(fmt.Sprintf(speedtest.Text(cfg.Language, "Threads: %d", "线程: %d"), threads))
			bus.Info(fmt.Sprintf(speedtest.Text(cfg.Language, "Limit: %s / %dms per worker", "上限: %s / 每线程 %dms"), cfg.Max, cfg.TimeoutMs))
		}

		var spec provider.RequestSpec
		var err error
		if dir == transfer.Download {
			spec, err = p.DownloadRequest(ctx, cfg.MaxBytes)
		} else {
			spec, err = p.UploadRequest(ctx, cfg.MaxBytes)
		}
		if err != nil {
			addWarning(&res, "round", err.Error())
			round := result.RoundResult{
				Name:          title,
				Direction:     string(dir),
				Threads:       threads,
				Status:        "failed",
				Error:         err.Error(),
				LoadedLatency: result.LatencyResult{Status: "unavailable"},
			}
			res.Rounds = append(res.Rounds, round)
			res.Degraded = true
			return
		}

		loadedProbe := latency.StartLoaded(ctx, func(ctx context.Context) (float64, error) {
			spec, err := p.LoadedLatencyRequest(ctx, string(dir))
			if err != nil {
				return -1, err
			}
			ms, _, err := performRequest(ctx, client, spec, p)
			return ms, err
		})

		xfer := transfer.Run(ctx, client, spec, dir, threads, time.Duration(cfg.TimeoutMs)*time.Millisecond,
			func(dir transfer.Direction, totalBytes int64, elapsed time.Duration, mbps float64) {
				if bus == nil {
					return
				}
				bus.Progress(speedtest.Text(cfg.Language, mapDirectionEN(dir), mapDirectionZH(dir)),
					fmt.Sprintf("%.1f Mbps  %s  %.1fs", mbps, speedconfig.HumanBytes(totalBytes), elapsed.Seconds()))
			},
		)
		loadedStats := loadedProbe.Stop()
		round := result.RoundResult{
			Name:          title,
			Direction:     string(dir),
			Threads:       threads,
			Status:        "ok",
			URL:           spec.URL,
			TotalBytes:    xfer.TotalBytes,
			DurationMs:    xfer.Duration.Milliseconds(),
			Mbps:          xfer.Mbps,
			FaultCount:    xfer.FaultCount,
			HadFault:      xfer.HadFault,
			LoadedLatency: latencyResult(loadedStats, speedtest.Text(cfg.Language, "No loaded latency samples collected.", "未采集到负载延迟样本。")),
		}
		if xfer.HadFault {
			round.Status = "degraded"
			round.Error = speedtest.Text(cfg.Language, "Network fault detected during transfer.", "传输过程中检测到网络故障。")
			res.Degraded = true
		}
		if xfer.TotalBytes == 0 {
			round.Status = "failed"
			if round.Error == "" {
				round.Error = speedtest.Text(cfg.Language, "Transfer did not complete successfully.", "传输未成功完成。")
			}
			res.Degraded = true
		}
		if round.LoadedLatency.Status != "ok" && round.Status == "ok" {
			round.Status = "degraded"
			res.Degraded = true
		}
		res.TotalBytes += xfer.TotalBytes
		res.Rounds = append(res.Rounds, round)
		if bus != nil {
			renderRound(bus, cfg, round)
		}
	}

	runRound(transfer.Download, 1, speedtest.Text(cfg.Language, "Download (single thread)", "下载（单线程）"))
	if cfg.Threads > 1 {
		runRound(transfer.Download, cfg.Threads, speedtest.Text(cfg.Language, "Download (multi-thread)", "下载（多线程）"))
	}
	runRound(transfer.Upload, 1, speedtest.Text(cfg.Language, "Upload (single thread)", "上传（单线程）"))
	if cfg.Threads > 1 {
		runRound(transfer.Upload, cfg.Threads, speedtest.Text(cfg.Language, "Upload (multi-thread)", "上传（多线程）"))
	}

	if interrupted(ctx) {
		return finalize(started, res, 130)
	}
	if bus != nil {
		renderSummary(bus, cfg, res)
	}
	exitCode := 0
	if res.Degraded {
		exitCode = 2
	}
	return finalize(started, res, exitCode)
}

func buildProvider(cfg *speedconfig.Config) (provider.Provider, error) {
	switch cfg.Provider {
	case "apple":
		return apple.New(), nil
	case "cloudflare":
		return cloudflare.New(cloudflare.NewMeasurementID()), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}

type discoveryResult struct {
	Host       string
	Candidates []candidate
	Selected   candidate
	Warnings   []result.Warning
}

func discoverCandidates(ctx context.Context, cfg *speedconfig.Config, p provider.Provider) (discoveryResult, error) {
	host := p.Host()
	out := discoveryResult{Host: host}
	if cfg.EndpointIP != "" {
		cand := buildCandidate(ctx, cfg, p, host, cfg.EndpointIP, "user")
		out.Candidates = []candidate{cand}
		out.Selected = cand
		return out, nil
	}

	ips, err := resolveAllIPsFn(ctx, host, cfg.DotServer)
	if err != nil {
		return out, err
	}
	seen := map[string]bool{}
	for _, ip := range ips {
		if ip == nil {
			continue
		}
		key := ip.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		out.Candidates = append(out.Candidates, buildCandidate(ctx, cfg, p, host, key, "dns"))
	}
	sort.SliceStable(out.Candidates, func(i, j int) bool {
		li, lj := out.Candidates[i], out.Candidates[j]
		if li.Status == "ok" && lj.Status != "ok" {
			return true
		}
		if li.Status != "ok" && lj.Status == "ok" {
			return false
		}
		return li.RTTMs < lj.RTTMs
	})
	for _, cand := range out.Candidates {
		if cand.Status == "ok" {
			out.Selected = cand
			break
		}
	}
	if out.Selected.IP == "" && len(out.Candidates) > 0 {
		out.Selected = out.Candidates[0]
		out.Warnings = append(out.Warnings, result.Warning{
			Code:    "degraded_selection",
			Message: speedtest.Text(cfg.Language, "no healthy endpoint candidate; continuing with the first resolved IP", "没有健康候选节点，继续使用首个解析 IP"),
		})
	}
	return out, nil
}

func buildCandidate(ctx context.Context, cfg *speedconfig.Config, p provider.Provider, host, ip, source string) candidate {
	cand := candidate{IP: ip, Source: source, Status: "degraded"}
	if !cfg.NoMetadata {
		cand.Desc = fetchIPDescFn(ctx, ip, cfg)
	}
	client, err := newPinnedClient(cfg, host, ip, time.Duration(cfg.TimeoutMs)*time.Millisecond)
	if err != nil {
		cand.Error = err.Error()
		return cand
	}
	spec, err := p.IdleLatencyRequest(ctx)
	if err != nil {
		cand.Error = err.Error()
		return cand
	}
	rtt, meta, err := performRequest(ctx, client, spec, p)
	if err != nil {
		cand.Error = err.Error()
		return cand
	}
	cand.RTTMs = rtt
	cand.Status = "ok"
	cand.Meta = meta
	return cand
}

func newPinnedClient(cfg *speedconfig.Config, host, ip string, timeout time.Duration) (*http.Client, error) {
	var localIP net.IP
	if ip != "" {
		resolved, err := resolveLocalBindIP(net.ParseIP(ip), cfg.SourceAddress, cfg.SourceDevice)
		if err != nil {
			return nil, err
		}
		localIP = resolved
	}
	return newHTTPClientFn(netx.Options{
		PinHost: host,
		PinIP:   ip,
		Timeout: timeout,
		LocalIP: localIP,
	}), nil
}

func resolveLocalBindIP(dstIP net.IP, sourceAddr, sourceDevice string) (net.IP, error) {
	if trimmed := strings.TrimSpace(sourceAddr); trimmed != "" {
		ip := net.ParseIP(trimmed)
		if ip == nil {
			return nil, fmt.Errorf("invalid source IP %q", sourceAddr)
		}
		if dstIP != nil && (ip.To4() == nil) != (dstIP.To4() == nil) {
			return nil, fmt.Errorf("source IP %q does not match the endpoint address family", sourceAddr)
		}
		return ip, nil
	}
	if strings.TrimSpace(sourceDevice) == "" || dstIP == nil {
		return nil, nil
	}
	dev, err := trace.ResolveSourceDevice(sourceDevice)
	if err != nil {
		return nil, err
	}
	resolved, err := trace.ResolveSourceDeviceAddr(dev, dstIP)
	if err != nil {
		return nil, err
	}
	if resolved == "" {
		return nil, fmt.Errorf("source device %q has no usable address for %s", sourceDevice, familyLabel(dstIP))
	}
	return net.ParseIP(resolved), nil
}

func familyLabel(dstIP net.IP) string {
	if dstIP != nil && dstIP.To4() == nil {
		return "IPv6"
	}
	return "IPv4"
}

func performRequest(ctx context.Context, client *http.Client, spec provider.RequestSpec, p provider.Provider) (float64, map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, spec.Method, spec.URL, nil)
	if err != nil {
		return 0, nil, err
	}
	for k, v := range spec.Headers {
		req.Header.Set(k, v)
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= http.StatusBadRequest {
		return 0, nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("read response body: %w", err)
	}
	meta := p.ParseMetadata(resp, body)
	return float64(time.Since(start).Nanoseconds()) / float64(time.Millisecond), meta, nil
}

func gatherConnectionInfo(ctx context.Context, cfg *speedconfig.Config, host string, selected candidate) result.ConnectionInfo {
	info := result.ConnectionInfo{
		Status:          "unavailable",
		MetadataEnabled: !cfg.NoMetadata,
		Host:            host,
		Client:          result.PeerInfo{Status: "unavailable"},
		Server:          result.PeerInfo{Status: "unavailable"},
	}
	if cfg.NoMetadata {
		return info
	}
	clientTarget := ""
	if selected.Meta != nil {
		if v, ok := selected.Meta["client_ip"].(string); ok {
			clientTarget = v
		}
	}
	info.Client = fetchPeerInfoFn(ctx, clientTarget, cfg)
	info.Server = fetchPeerInfoFn(ctx, selected.IP, cfg)
	if info.Server.Status == "ok" && info.Server.IP == "" {
		info.Server.IP = selected.IP
	}
	if info.Server.Location == "" && selected.Desc != "" {
		info.Server.Location = selected.Desc
	}
	if info.Client.Status == "ok" {
		info.Client.ProviderMeta = extractClientMeta(selected.Meta)
	}
	if info.Server.Status == "ok" {
		info.Server.ProviderMeta = extractServerMeta(selected.Meta)
	}
	switch {
	case info.Client.Status == "ok" && info.Server.Status == "ok":
		info.Status = "ok"
	case info.Client.Status == "ok" || info.Server.Status == "ok":
		info.Status = "degraded"
	default:
		info.Status = "unavailable"
	}
	return info
}

func extractClientMeta(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return nil
	}
	out := map[string]any{}
	if v, ok := meta["client_ip"]; ok {
		out["client_ip"] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func extractServerMeta(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return nil
	}
	out := map[string]any{}
	for _, key := range []string{"colo", "city", "country", "postal_code", "timezone", "asn", "cf_ray", "via", "x_cache", "cdn_uuid"} {
		if v, ok := meta[key]; ok {
			out[key] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func renderSelection(bus *render.Bus, ctx context.Context, cfg *speedconfig.Config, discovery *discoveryResult, allowPrompt bool) {
	bus.Header(speedtest.Text(cfg.Language, "Endpoint Selection", "节点选择"))
	bus.Info(speedtest.Text(cfg.Language, "Host: ", "主机: ") + discovery.Host)
	if len(discovery.Candidates) == 0 {
		bus.Warn(speedtest.Text(cfg.Language, "No endpoint candidates found.", "未找到候选节点。"))
		return
	}
	bus.Info(speedtest.Text(cfg.Language, "Available endpoints:", "可用节点:"))
	for i, cand := range discovery.Candidates {
		label := cand.Desc
		if label == "" {
			label = speedtest.Text(cfg.Language, "lookup failed", "查询失败")
		}
		if cand.Status == "ok" {
			bus.Info(fmt.Sprintf("  %d) %s  %.2fms  %s", i+1, cand.IP, cand.RTTMs, label))
		} else {
			bus.Info(fmt.Sprintf("  %d) %s  %s  %s", i+1, cand.IP, speedtest.Text(cfg.Language, "unavailable", "不可用"), label))
		}
	}
	if len(discovery.Candidates) > 1 && allowPrompt {
		bus.Flush()
		choice, cancelled := promptChoice(ctx, len(discovery.Candidates), cfg.Language)
		if !cancelled {
			discovery.Selected = discovery.Candidates[choice]
		}
	}
	if discovery.Selected.IP != "" {
		desc := discovery.Selected.Desc
		if desc == "" {
			desc = speedtest.Text(cfg.Language, "selected", "已选择")
		}
		bus.Info(fmt.Sprintf(speedtest.Text(cfg.Language, "Selected endpoint: %s (%s)", "已选择节点: %s (%s)"), discovery.Selected.IP, desc))
	}
}

func renderConnectionInfo(bus *render.Bus, cfg *speedconfig.Config, info result.ConnectionInfo) {
	bus.Header(speedtest.Text(cfg.Language, "Connection Information", "连接信息"))
	if !info.MetadataEnabled {
		bus.Info(speedtest.Text(cfg.Language, "Metadata lookup disabled.", "已禁用元数据查询。"))
		return
	}
	renderPeer(bus, cfg, speedtest.Text(cfg.Language, "Client", "客户端"), info.Client)
	renderPeer(bus, cfg, speedtest.Text(cfg.Language, "Server", "服务端"), info.Server)
}

func renderPeer(bus *render.Bus, cfg *speedconfig.Config, label string, peer result.PeerInfo) {
	if peer.Status != "ok" {
		bus.KV(label, speedtest.Text(cfg.Language, "unavailable", "不可用"))
		return
	}
	bus.KV(label, fmt.Sprintf("%s  (%s)", fallback(peer.IP), fallback(peer.ISP)))
	bus.KV("  ASN", fallback(peer.ASN))
	bus.KV(speedtest.Text(cfg.Language, "  Location", "  位置"), fallback(peer.Location))
}

func renderLatency(bus *render.Bus, cfg *speedconfig.Config, lr result.LatencyResult) {
	if lr.Status != "ok" {
		bus.Warn(fallback(lr.Error))
		return
	}
	bus.Result(fmt.Sprintf(speedtest.Text(cfg.Language,
		"%.2f ms median  (min %.2f / avg %.2f / max %.2f)  jitter %.2f ms",
		"%.2f 毫秒 中位数  (最小 %.2f / 平均 %.2f / 最大 %.2f)  抖动 %.2f 毫秒"),
		value(lr.MedianMs), value(lr.MinMs), value(lr.AvgMs), value(lr.MaxMs), value(lr.JitterMs)))
}

func renderRound(bus *render.Bus, cfg *speedconfig.Config, round result.RoundResult) {
	if round.Threads <= 1 {
		bus.Result(fmt.Sprintf(speedtest.Text(cfg.Language, "%.0f Mbps  (%s in %.1fs)", "%.0f Mbps  (%s，耗时 %.1fs)"),
			round.Mbps, speedconfig.HumanBytes(round.TotalBytes), float64(round.DurationMs)/1000))
	} else {
		bus.Result(fmt.Sprintf(speedtest.Text(cfg.Language, "%.0f Mbps  (%s in %.1fs, %d threads)", "%.0f Mbps  (%s，耗时 %.1fs，%d 线程)"),
			round.Mbps, speedconfig.HumanBytes(round.TotalBytes), float64(round.DurationMs)/1000, round.Threads))
	}
	if round.Error != "" {
		bus.Warn(round.Error)
	}
	if round.LoadedLatency.Status == "ok" {
		bus.Info(fmt.Sprintf(speedtest.Text(cfg.Language, "Loaded latency: %.2f ms  (jitter %.2f ms)", "负载延迟: %.2f 毫秒  (抖动 %.2f 毫秒)"),
			value(round.LoadedLatency.MedianMs), value(round.LoadedLatency.JitterMs)))
	} else if round.LoadedLatency.Error != "" {
		bus.Warn(round.LoadedLatency.Error)
	}
}

func renderSummary(bus *render.Bus, cfg *speedconfig.Config, res result.RunResult) {
	bus.Line()
	bus.Banner(speedtest.Text(cfg.Language, "Summary", "测速汇总"))
	bus.Line()
	if res.IdleLatency.Status == "ok" {
		bus.KV(speedtest.Text(cfg.Language, "Idle Latency", "空载延迟"),
			fmt.Sprintf(speedtest.Text(cfg.Language, "%.2f ms  (jitter %.2f ms)", "%.2f 毫秒  (抖动 %.2f 毫秒)"),
				value(res.IdleLatency.MedianMs), value(res.IdleLatency.JitterMs)))
	}
	bus.KV(speedtest.Text(cfg.Language, "Data Used", "消耗流量"), speedconfig.HumanBytes(res.TotalBytes))
	if res.Degraded {
		bus.Warn(speedtest.Text(cfg.Language, "Completed with degraded results.", "测速完成，但结果存在降级。"))
	}
}

func latencyResult(stats latency.Stats, emptyMessage string) result.LatencyResult {
	if stats.N == 0 {
		return result.LatencyResult{Status: "unavailable", Error: emptyMessage}
	}
	return result.LatencyResult{
		Status:   "ok",
		Samples:  stats.N,
		MinMs:    ptr(stats.Min),
		AvgMs:    ptr(stats.Avg),
		MedianMs: ptr(stats.Median),
		MaxMs:    ptr(stats.Max),
		JitterMs: ptr(stats.Jitter),
	}
}

func finalize(started time.Time, res result.RunResult, exitCode int) result.RunResult {
	res.ExitCode = exitCode
	res.DurationMs = time.Since(started).Milliseconds()
	if exitCode == 130 {
		res.Degraded = true
		addWarning(&res, "interrupted", "interrupted")
	}
	return res
}

func interrupted(ctx context.Context) bool {
	return ctx != nil && ctx.Err() != nil
}

func addWarning(res *result.RunResult, code, message string) {
	if res == nil {
		return
	}
	res.Warnings = append(res.Warnings, result.Warning{Code: code, Message: message})
}

func selectedEndpoint(c candidate) result.SelectedEndpoint {
	out := result.SelectedEndpoint{
		IP:          c.IP,
		Description: c.Desc,
		Source:      c.Source,
		Status:      c.Status,
	}
	if c.RTTMs > 0 {
		out.RTTMs = ptr(c.RTTMs)
	}
	return out
}

func candidateResults(cands []candidate) []result.CandidateResult {
	out := make([]result.CandidateResult, 0, len(cands))
	for _, c := range cands {
		entry := result.CandidateResult{
			IP:          c.IP,
			Description: c.Desc,
			Source:      c.Source,
			Status:      c.Status,
			Error:       c.Error,
		}
		if c.RTTMs > 0 {
			entry.RTTMs = ptr(c.RTTMs)
		}
		out = append(out, entry)
	}
	return out
}

func mapDirectionZH(dir transfer.Direction) string {
	if dir == transfer.Upload {
		return "上传"
	}
	return "下载"
}

func mapDirectionEN(dir transfer.Direction) string {
	if dir == transfer.Upload {
		return "Upload"
	}
	return "Download"
}

func ptr(v float64) *float64 { return &v }

func value(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func fallback(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}

func openPromptInput() (*os.File, error) {
	for _, path := range []string{"/dev/tty", "CONIN$", "/dev/stdin"} {
		file, err := os.Open(path)
		if err == nil {
			return file, nil
		}
	}
	return nil, fmt.Errorf("interactive input not available")
}

func promptChoice(ctx context.Context, count int, lang string) (int, bool) {
	fmt.Fprintf(os.Stderr, "  [?] %s", fmt.Sprintf(speedtest.Text(lang, "Select endpoint [1-%d, Enter=1]: ", "选择节点 [1-%d，回车=1]: "), count))
	tty, err := openPromptInputFn()
	if err != nil {
		return 0, false
	}
	defer func() {
		_ = tty.Close()
	}()
	type readResult struct {
		line string
		err  error
	}
	ch := make(chan readResult, 1)
	go func() {
		reader := bufio.NewReader(tty)
		line, err := reader.ReadString('\n')
		ch <- readResult{line: line, err: err}
	}()
	select {
	case <-ctx.Done():
		return 0, true
	case res := <-ch:
		if res.err != nil && res.line == "" {
			return 0, false
		}
		return parseChoice(res.line, count), false
	}
}

func parseChoice(line string, count int) int {
	line = strings.TrimSpace(line)
	if line == "" {
		return 0
	}
	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > count {
		return 0
	}
	return n - 1
}

func MarshalJSON(res result.RunResult) ([]byte, error) {
	return json.Marshal(res)
}
