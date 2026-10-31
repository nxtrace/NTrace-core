package trace

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/gopacket/layers"

	"github.com/nxtrace/NTrace-core/trace/internal"
	"github.com/nxtrace/NTrace-core/util"
)

// ---------------------------------------------------------------------------
// MTR 长驻运行器
// ---------------------------------------------------------------------------

// MTROptions 控制 MTR 连续探测行为。
type MTROptions struct {
	// Interval 每轮之间的等待间隔（默认 1s）。
	// 仅在 legacy round-based 模式（Web MTR）使用。
	Interval time.Duration
	// MaxRounds 最大轮次，0 表示无限运行直到取消。
	// 仅在 legacy round-based 模式使用。
	MaxRounds int
	// HopInterval 同一 TTL 两次探测之间的间隔（per-hop 调度模式）。
	// > 0 时启用 per-hop 调度，忽略 Interval / MaxRounds。
	HopInterval time.Duration
	// MaxPerHop 每个 TTL 最大探测次数，0 表示无限。
	MaxPerHop int
	// IsPaused 可选：返回 true 时暂停探测（轮询检查）。
	IsPaused func() bool
	// IsResetRequested 可选：返回 true（原子消费）时重置统计。
	IsResetRequested func() bool
	// ProgressThrottle 流式预览最小刷新间隔（默认 200ms）。
	ProgressThrottle time.Duration
	// AsyncMetadata causes interactive per-hop MTR to render bare hops first,
	// then patch Geo/RDNS metadata into existing rows asynchronously.
	AsyncMetadata bool
	// OnProbe is called for each per-hop probe that is counted into Snt.
	// It is only used by per-hop MTR; discarded probes are not emitted.
	OnProbe func(MTRProbeEvent)
	// OnPathEnd is called whenever the semantic path edge changes. A nil value
	// means that a provisional unreachable edge was reopened or reset.
	OnPathEnd func(*StopReason)
	// OnEvent records applied per-hop session changes. An error stops probing.
	OnEvent func(MTRSessionEvent) error
	// OnProbeError handles retry diagnostics instead of writing them to stderr.
	OnProbeError func(error)
}

// MTROnSnapshot 每轮完成后的回调，用于刷新 CLI 表格。
// iteration 是当前轮次（从 1 开始），stats 是截至当前的聚合快照。
type MTROnSnapshot func(iteration int, stats []MTRHopStat)

// MTRProbeEvent is a minimal per-probe event emitted after scheduler accounting.
type MTRProbeEvent struct {
	TTL       int
	Success   bool
	RTT       time.Duration
	Timestamp time.Time
	Response  *MTRProbeResponse
}

// mtrBackoffCfg 控制连续错误时的指数退避行为。
type mtrBackoffCfg struct {
	Initial   time.Duration
	Max       time.Duration
	MaxConsec int
}

var defaultBackoff = mtrBackoffCfg{
	Initial:   500 * time.Millisecond,
	Max:       30 * time.Second,
	MaxConsec: 10,
}

// mtrProber 抽象一轮探测，允许测试注入 mock。
type mtrProber interface {
	probeRound(ctx context.Context) (*Result, error)
	close()
}

// mtrResetter 可选接口：重置统计时清除 prober 内部缓存。
type mtrResetter interface {
	resetFinalTTL()
}

// mtrPeeker 可选接口：支持流式预览探测进度。
// 引擎在 probeRound 执行期间，外部可调用 peekPartialResult 获取当前已收到的部分响应。
type mtrPeeker interface {
	peekPartialResult() *Result
}

// RunMTR 启动 MTR 连续探测模式。
//
// 当 opts.HopInterval > 0 时使用 per-hop 独立调度（CLI MTR 模式）：
//
//	每个 TTL 独立计时，完成后等 HopInterval 再发下一个。
//
// 当 opts.HopInterval == 0 时使用 legacy round-based 调度（Web MTR 兼容）：
//
//	ICMP 使用持久 raw socket 跨轮复用；TCP/UDP 以 per-round Traceroute 回退。
func RunMTR(ctx context.Context, method Method, baseConfig Config, opts MTROptions, onSnapshot MTROnSnapshot) error {
	if opts.HopInterval > 0 {
		return runMTRPerHop(ctx, method, baseConfig, opts, onSnapshot)
	}
	if opts.OnEvent != nil {
		return fmt.Errorf("mtr: session recording requires per-hop scheduling")
	}
	return runMTRRoundBased(ctx, method, baseConfig, opts, onSnapshot)
}

// runMTRPerHop 使用 per-hop 独立调度模式。
func runMTRPerHop(ctx context.Context, method Method, baseConfig Config, opts MTROptions, onSnapshot MTROnSnapshot) error {
	normalizeRuntimeConfig(&baseConfig)
	baseConfig.Context = ctx
	var sourceErr error
	baseConfig, sourceErr = PrepareProbeSourceConfig(method, baseConfig)
	if sourceErr != nil {
		return sourceErr
	}

	baseConfig.NumMeasurements = 1
	baseConfig.MaxAttempts = 1
	baseConfig.RealtimePrinter = nil
	baseConfig.AsyncPrinter = nil

	if baseConfig.MaxHops == 0 {
		baseConfig.MaxHops = 30
	}
	if baseConfig.ICMPMode <= 0 && util.EnvICMPMode > 0 {
		baseConfig.ICMPMode = util.EnvICMPMode
	}
	switch baseConfig.ICMPMode {
	case 0, 1, 2:
	default:
		baseConfig.ICMPMode = 0
	}

	agg := NewMTRAggregator()
	var prober mtrTTLProber

	if method == ICMPTrace {
		engine, err := newMTRICMPEngine(baseConfig)
		if err != nil {
			return fmt.Errorf("mtr: %w", err)
		}
		prober = engine
	} else {
		prober = &mtrFallbackTTLProber{method: method, config: baseConfig, asyncMetadata: opts.AsyncMetadata}
	}

	return runMTRScheduler(ctx, prober, agg, mtrSchedulerConfig{
		BeginHop:         baseConfig.BeginHop,
		MaxHops:          baseConfig.MaxHops,
		HopInterval:      opts.HopInterval,
		Timeout:          baseConfig.Timeout,
		MaxPerHop:        opts.MaxPerHop,
		ParallelRequests: baseConfig.ParallelRequests,
		ProgressThrottle: opts.ProgressThrottle,
		FillGeo:          true,
		AsyncMetadata:    opts.AsyncMetadata,
		BaseConfig:       baseConfig,
		IsPaused:         opts.IsPaused,
		IsResetRequested: opts.IsResetRequested,
		OnPathEnd:        opts.OnPathEnd,
		OnEvent:          opts.OnEvent,
		OnProbeError:     opts.OnProbeError,
		StartErrorPrefix: "mtr",
	}, onSnapshot, mtrProbeCallbackFromOptions(opts))
}

func reportMTRProbeError(handler func(error), consecutive, limit int, err error) {
	message := fmt.Errorf("mtr: probe error (%d/%d): %w", consecutive, limit, err)
	if handler != nil {
		handler(message)
		return
	}
	fmt.Fprintln(os.Stderr, message)
}

func mtrProbeCallbackFromOptions(opts MTROptions) func(mtrProbeResult, int, time.Time) {
	if opts.OnProbe == nil {
		return nil
	}
	return func(result mtrProbeResult, _ int, at time.Time) {
		opts.OnProbe(mtrProbeEventFromResult(result, at))
	}
}

// runMTRRoundBased 使用 legacy round-based 调度模式（Web MTR 兼容）。
func runMTRRoundBased(ctx context.Context, method Method, baseConfig Config, opts MTROptions, onSnapshot MTROnSnapshot) error {
	normalizeRuntimeConfig(&baseConfig)
	baseConfig.Context = ctx
	var sourceErr error
	baseConfig, sourceErr = PrepareProbeSourceConfig(method, baseConfig)
	if sourceErr != nil {
		return sourceErr
	}

	if opts.Interval <= 0 {
		opts.Interval = time.Second
	}

	// MTR：每轮每 hop 仅一个探测包
	baseConfig.NumMeasurements = 1
	baseConfig.MaxAttempts = 1
	// 注意：不覆盖 ParallelRequests——尊重用户 --parallel-requests 设定
	baseConfig.RealtimePrinter = nil
	baseConfig.AsyncPrinter = nil

	// 与 Traceroute() 保持一致的默认值
	if baseConfig.MaxHops == 0 {
		baseConfig.MaxHops = 30
	}
	if baseConfig.ICMPMode <= 0 && util.EnvICMPMode > 0 {
		baseConfig.ICMPMode = util.EnvICMPMode
	}
	switch baseConfig.ICMPMode {
	case 0, 1, 2:
	default:
		baseConfig.ICMPMode = 0
	}

	agg := NewMTRAggregator()

	if method == ICMPTrace {
		engine, err := newMTRICMPEngine(baseConfig)
		if err != nil {
			return fmt.Errorf("mtr: %w", err)
		}
		return mtrLoop(ctx, engine, baseConfig, opts, agg, onSnapshot, true, nil)
	}

	prober := &mtrFallbackProber{method: method, config: baseConfig}
	// Traceroute 内部已做 geo/rdns 查询，无需再填充
	return mtrLoop(ctx, prober, baseConfig, opts, agg, onSnapshot, false, nil)
}

// ---------------------------------------------------------------------------
// 主循环（ICMP 持久引擎 / TCP·UDP 回退共用）
// ---------------------------------------------------------------------------

func mtrLoop(
	ctx context.Context,
	prober mtrProber,
	config Config,
	opts MTROptions,
	agg *MTRAggregator,
	onSnapshot MTROnSnapshot,
	fillGeo bool,
	bo *mtrBackoffCfg,
) error {
	session := newMTRWorkerSession(ctx)
	defer session.shutdown(prober.close)
	if starter, ok := prober.(mtrSessionStarter); ok {
		if err := starter.startMTRSession(session); err != nil {
			return fmt.Errorf("mtr: %w", err)
		}
	}
	rt := newMTRLoopRuntime(session, prober, config, opts, agg, onSnapshot, fillGeo, bo)
	return rt.run()
}

// mtrFillGeoRDNS 并发查询 Result 中各 hop 的地理信息与反向 DNS。
// fetchIPData 内部有 singleflight + geoCache，重复 IP 不会重复查询。
func mtrFillGeoRDNS(workers *mtrWorkerSession, res *Result, config Config) {
	config.Context = workers.ctx
	var wg sync.WaitGroup
	for idx := range res.Hops {
		for j := range res.Hops[idx] {
			h := &res.Hops[idx][j]
			if !h.Success || h.Address == nil {
				continue
			}
			h.Lang = config.Lang
			hop := h
			worker := func() {
				defer wg.Done()
				_ = hop.fetchIPData(config)
			}
			wg.Add(1)
			workers.Go("mtr.metadata.sync", worker)
		}
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// 持久 ICMP 引擎（mtr 风格：raw socket 只创建一次，跨轮复用）
// ---------------------------------------------------------------------------

type mtrICMPEngine struct {
	config         Config
	spec           atomic.Pointer[internal.ICMPSpec]
	specMu         sync.Mutex
	echoID         atomic.Int32
	srcIP          net.IP
	ipVer          int
	workers        *mtrWorkerSession
	listenerCancel context.CancelFunc
	listenerDone   chan struct{}

	// 单调递增序列号，避免跨轮 seq 冲突
	seqCounter atomic.Uint32

	// per-round 探针/响应匹配
	mu       sync.Mutex
	sentAt   map[int]mtrProbeMeta // seq → probe metadata
	replied  map[int]*mtrProbeReply
	notifyCh chan struct{}

	// 当前轮次 ID，用于丢弃过期响应
	roundID atomic.Uint32

	// 已知目的地 TTL（-1 = 未知），跨轮保留以减少无效探测
	knownFinalTTL atomic.Int32
	// 当前轮次内发现的目的地 TTL（-1 = 本轮未发现）
	roundFinalTTL atomic.Int32

	// 流式预览状态（peekPartialResult 使用，受 mu 保护）
	curTtlSeq       map[int]int
	curBeginHop     int
	curEffectiveMax int

	// Per-probe notification channels for ProbeTTL（受 mu 保护）。
	probeNotify map[int]chan struct{} // seq → done chan

	// sendMu orders sequence allocation, registration and sending before a
	// sequence-wrap rotation. Reply waiting and Close must not hold it.
	sendMu sync.Mutex
}

// mtrProbeMeta 记录已发送探针的元信息，用于响应匹配。
type mtrProbeMeta struct {
	ttl     int
	start   time.Time
	roundID uint32
}

type mtrProbeReply struct {
	peer     net.Addr
	rtt      time.Duration
	mpls     []string
	response *MTRProbeResponse
}

func newMTRICMPEngine(config Config) (*mtrICMPEngine, error) {
	ipVer := 4
	if config.DstIP.To4() == nil {
		ipVer = 6
	}

	var srcAddr net.IP
	if config.SrcAddr != "" {
		srcAddr = net.ParseIP(config.SrcAddr)
		if ipVer == 4 && srcAddr != nil {
			srcAddr = srcAddr.To4()
		}
	}

	var srcIP net.IP
	if usesPolicyRouteSource(config) {
		var err error
		srcIP, err = resolveProbeSource(ICMPTrace, &config, srcAddr)
		if err != nil {
			return nil, wrapProbeSetupError(err)
		}
	} else if ipVer == 6 {
		srcIP, _ = util.LocalIPPortv6(config.DstIP, srcAddr, "icmp6")
	} else {
		srcIP, _ = util.LocalIPPort(config.DstIP, srcAddr, "icmp")
	}
	if srcIP == nil {
		return nil, fmt.Errorf("cannot determine local IP for MTR ICMP")
	}

	return newMTRICMPEngineState(config, ipVer, srcIP), nil
}

func newMTRICMPEngineState(config Config, ipVer int, srcIP net.IP) *mtrICMPEngine {
	engine := &mtrICMPEngine{
		config: config,
		ipVer:  ipVer,
		srcIP:  srcIP,
	}
	engine.echoID.Store(int32(newMTREchoID()))
	engine.knownFinalTTL.Store(-1)
	engine.roundFinalTTL.Store(-1)
	return engine
}

func newMTREchoID() int {
	return (rand.IntN(256) << 8) | (os.Getpid() & 0xFF)
}

// startMTRSession 创建持久 ICMP 套接字，并把 listener 交给会话统一等待。
func (e *mtrICMPEngine) startMTRSession(workers *mtrWorkerSession) error {
	e.specMu.Lock()
	defer e.specMu.Unlock()
	e.workers = workers
	ctx := workers.ctx
	spec := internal.NewICMPSpec(e.ipVer, e.config.ICMPMode, int(e.echoID.Load()), e.srcIP, e.config.DstIP)
	applyICMPSourceDevice(spec, e.config.OSType, e.config.SourceDevice)
	spec.FWMark, spec.FWMarkSet = e.config.FWMark, e.config.FWMarkSet
	if err := spec.InitICMP(); err != nil {
		spec.Close()
		return wrapProbeSetupError(err)
	}
	e.spec.Store(spec)

	e.notifyCh = make(chan struct{}, 1)
	e.sentAt = make(map[int]mtrProbeMeta)
	e.replied = make(map[int]*mtrProbeReply)
	e.probeNotify = make(map[int]chan struct{})

	ready := e.startICMPListener(spec)
	if err := waitMTRListenerReady(ctx, ready, "ICMP listener startup timeout"); err != nil {
		return err
	}
	return waitMTRTimer(ctx, 100*time.Millisecond)
}

func (e *mtrICMPEngine) close() {
	e.specMu.Lock()
	defer e.specMu.Unlock()
	e.stopICMPListener()
}

// Caller holds specMu; each rotation owns and joins its listener.
func (e *mtrICMPEngine) startICMPListener(spec *internal.ICMPSpec) chan struct{} {
	ctx, cancel := context.WithCancel(e.workers.ctx)
	done, ready := make(chan struct{}), make(chan struct{})
	e.listenerCancel, e.listenerDone = cancel, done
	e.workers.Go("mtr.icmp-listener", func() {
		defer close(done)
		if err := spec.ListenICMP(ctx, ready, e.onICMP); err != nil && ctx.Err() == nil {
			e.workers.cancel(wrapProbeSetupError(err))
		}
	})
	return ready
}

func (e *mtrICMPEngine) stopICMPListener() {
	if e.listenerCancel != nil {
		e.listenerCancel()
	}
	if spec := e.spec.Swap(nil); spec != nil {
		spec.Close()
	}
	if e.listenerDone != nil {
		<-e.listenerDone
	}
	e.listenerCancel, e.listenerDone = nil, nil
}

// resetFinalTTL 清除已知目的地 TTL 缓存（r 键重置统计时调用）。
func (e *mtrICMPEngine) resetFinalTTL() {
	e.knownFinalTTL.Store(-1)
}

// peekPartialResult 返回当前轮次已收到的部分探测结果（用于流式预览）。
// 在 probeRound 运行期间由外部 ticker 调用，与 probeRound 共享 mu 保护。
func (e *mtrICMPEngine) peekPartialResult() *Result {
	e.mu.Lock()
	defer e.mu.Unlock()

	beginHop := e.curBeginHop
	effectiveMax := e.curEffectiveMax
	if effectiveMax <= 0 || e.curTtlSeq == nil {
		return nil
	}

	// 若已检测到目的地，缩小预览范围
	if rf := e.roundFinalTTL.Load(); rf > 0 && int(rf) < effectiveMax {
		effectiveMax = int(rf)
	}

	res := &Result{Hops: make([][]Hop, effectiveMax)}
	for ttl := beginHop; ttl <= effectiveMax; ttl++ {
		idx := ttl - 1
		seq, sent := e.curTtlSeq[ttl]
		if !sent {
			// 尚未发送，保持 nil 槽位让聚合器跳过
			continue
		}
		if reply, ok := e.replied[seq]; ok {
			res.Hops[idx] = []Hop{{
				Success: true,
				Address: reply.peer,
				TTL:     ttl,
				RTT:     reply.rtt,
				MPLS:    reply.mpls,
			}}
		} else {
			// 已发送但未收到响应，显示为超时
			res.Hops[idx] = []Hop{{
				Success: false,
				Address: nil,
				TTL:     ttl,
				RTT:     0,
			}}
		}
	}
	return res
}

// seqWillWrap 判断再发 probeCount 个探针后 16 位 wire seq 是否会回卷。
// probeCount <= 0 时不发包，不可能回卷。
func seqWillWrap(seqCounter uint32, probeCount int) bool {
	if probeCount <= 0 {
		return false
	}
	return (seqCounter&0xFFFF)+uint32(probeCount) > 0xFFFF
}

// rotateEngine 关闭旧 ICMP 监听器，生成新 echoID 并重建引擎。
// 新 listener 过滤新 echoID，旧 echoID 的迟到回包在协议层即被丢弃，
// 从而彻底消除 seq 16 位回卷导致的跨轮误匹配。
func (e *mtrICMPEngine) rotateEngine(ctx context.Context) error {
	e.specMu.Lock()
	defer e.specMu.Unlock()
	e.stopICMPListener()

	e.echoID.Store(int32(newMTREchoID()))
	e.seqCounter.Store(0)

	spec := internal.NewICMPSpec(e.ipVer, e.config.ICMPMode, int(e.echoID.Load()), e.srcIP, e.config.DstIP)
	applyICMPSourceDevice(spec, e.config.OSType, e.config.SourceDevice)
	spec.FWMark, spec.FWMarkSet = e.config.FWMark, e.config.FWMarkSet
	if err := spec.InitICMP(); err != nil {
		spec.Close()
		return wrapProbeSetupError(err)
	}
	e.spec.Store(spec)

	e.mu.Lock()
	// Notify any ProbeTTL waiters about rotation (they'll see no reply)
	for seq, ch := range e.probeNotify {
		close(ch)
		delete(e.probeNotify, seq)
	}
	e.notifyCh = make(chan struct{}, 1)
	e.sentAt = make(map[int]mtrProbeMeta)
	e.replied = make(map[int]*mtrProbeReply)
	e.probeNotify = make(map[int]chan struct{})
	e.mu.Unlock()

	if e.workers == nil {
		return wrapProbeSetupError(fmt.Errorf("ICMP listener has no MTR worker session"))
	}
	ready := e.startICMPListener(spec)
	return waitMTRListenerReady(ctx, ready, "ICMP listener restart timeout on echoID rotation")
}

func waitMTRListenerReady(ctx context.Context, ready <-chan struct{}, timeoutMessage string) error {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-timer.C:
		return wrapProbeSetupError(fmt.Errorf("%s", timeoutMessage))
	}
}

func waitMTRTimer(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-timer.C:
		return nil
	}
}

// onICMP 是 ListenICMP 的回调：将响应匹配到已发送的探针。
func (e *mtrICMPEngine) onICMP(msg internal.ReceivedMessage, finish time.Time, seq int) {
	e.mu.Lock()
	start, ok := e.sentAt[seq]
	if !ok {
		e.mu.Unlock()
		return
	}
	if e.shouldDiscardProbeReplyLocked(seq, start, finish) {
		e.mu.Unlock()
		return
	}

	rtt := finish.Sub(start.start)
	e.storeProbeReplyLocked(seq, msg, rtt)
	finalTTL := e.detectRoundFinalTTLCandidate(e.replied[seq].response, start.ttl)
	e.mu.Unlock()

	e.updateRoundFinalTTL(finalTTL)
	e.signalReplyReady()
}

func (e *mtrICMPEngine) shouldDiscardProbeReplyLocked(seq int, start mtrProbeMeta, finish time.Time) bool {
	if start.roundID != e.roundID.Load() {
		e.discardProbeLocked(seq)
		return true
	}
	if !e.validProbeRTT(finish.Sub(start.start)) {
		e.discardProbeLocked(seq)
		return true
	}
	return false
}

func (e *mtrICMPEngine) validProbeRTT(rtt time.Duration) bool {
	maxRTT := e.config.Timeout
	if maxRTT <= 0 {
		maxRTT = 2 * time.Second
	}
	// A valid reply can arrive within one clock tick, especially on Windows.
	return rtt >= 0 && rtt <= maxRTT
}

func (e *mtrICMPEngine) discardProbeLocked(seq int) {
	delete(e.sentAt, seq)
	e.closeProbeNotifyLocked(seq)
}

func (e *mtrICMPEngine) storeProbeReplyLocked(seq int, msg internal.ReceivedMessage, rtt time.Duration) {
	e.replied[seq] = &mtrProbeReply{
		peer:     msg.Peer,
		rtt:      rtt,
		mpls:     extractMPLS(msg, e.config.DisableMPLS),
		response: mtrProbeResponseFromProbe(probeResponseFromICMPForMethod(msg.ICMP, ICMPTrace)),
	}
	delete(e.sentAt, seq)
	e.closeProbeNotifyLocked(seq)
}

func (e *mtrICMPEngine) closeProbeNotifyLocked(seq int) {
	if ch, ok := e.probeNotify[seq]; ok {
		close(ch)
		delete(e.probeNotify, seq)
	}
}

func (e *mtrICMPEngine) detectRoundFinalTTLCandidate(response *MTRProbeResponse, ttl int) int32 {
	// Only destination evidence is sticky inside the round engine. Unreachable
	// evidence is provisional and is maintained by the outer path tracker so a
	// later transit reply can reopen the scan without losing higher-TTL data.
	if response == nil || response.Kind != MTRResponseDestination {
		return -1
	}
	return int32(ttl)
}

func mtrPeerIP(addr net.Addr) net.IP {
	switch a := addr.(type) {
	case *net.IPAddr:
		return a.IP
	case *net.UDPAddr:
		return a.IP
	case *net.TCPAddr:
		return a.IP
	default:
		return nil
	}
}

func (e *mtrICMPEngine) updateRoundFinalTTL(ttl int32) {
	if ttl < 0 {
		return
	}
	curFinal := e.roundFinalTTL.Load()
	if curFinal < 0 || ttl < curFinal {
		e.roundFinalTTL.Store(ttl)
	}
}

func (e *mtrICMPEngine) signalReplyReady() {
	e.mu.Lock()
	notifyCh := e.notifyCh
	e.mu.Unlock()
	select {
	case notifyCh <- struct{}{}:
	default:
	}
}

// sendProbe 发送一个 ICMP echo（IPv4 或 IPv6），返回发送时间戳。
func (e *mtrICMPEngine) sendProbe(ctx context.Context, ttl, seq int) (time.Time, error) {
	spec := e.spec.Load()
	if spec == nil {
		return time.Time{}, fmt.Errorf("ICMP listener is closed")
	}
	payloadSize := resolveProbePayloadSize(ICMPTrace, e.config.DstIP, e.config.PktSize, e.config.RandomPacketSize)
	payload := make([]byte, payloadSize)
	if len(payload) >= 3 {
		copy(payload[len(payload)-3:], []byte{'n', 't', 'r'})
	}

	if e.ipVer == 4 {
		ipHdr := &layers.IPv4{
			Version:  4,
			SrcIP:    e.srcIP,
			DstIP:    e.config.DstIP,
			Protocol: layers.IPProtocolICMPv4,
			TTL:      uint8(ttl),
			TOS:      uint8(e.config.TOS),
		}
		icmpHdr := &layers.ICMPv4{
			TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0),
			Id:       uint16(e.echoID.Load()),
			Seq:      uint16(seq),
		}
		return spec.SendICMP(ctx, ipHdr, icmpHdr, nil, payload)
	}

	// IPv6
	ipHdr := &layers.IPv6{
		Version:      6,
		SrcIP:        e.srcIP,
		DstIP:        e.config.DstIP,
		NextHeader:   layers.IPProtocolICMPv6,
		HopLimit:     uint8(ttl),
		TrafficClass: uint8(e.config.TOS),
	}
	icmpHdr := &layers.ICMPv6{
		TypeCode: layers.CreateICMPv6TypeCode(layers.ICMPv6TypeEchoRequest, 0),
	}
	if err := icmpHdr.SetNetworkLayerForChecksum(ipHdr); err != nil {
		return time.Time{}, fmt.Errorf("SetNetworkLayerForChecksum: %w", err)
	}
	icmpEcho := &layers.ICMPv6Echo{
		Identifier: uint16(e.echoID.Load()),
		SeqNumber:  uint16(seq),
	}
	return spec.SendICMP(ctx, ipHdr, icmpHdr, icmpEcho, payload)
}

// probeRound 执行一轮持久探测：对每个 TTL 发送一个 ICMP echo，
// 收集响应后构造与 Traceroute 兼容的 *Result。
// 若已知目的地 TTL 则提前停止发送。
func (e *mtrICMPEngine) probeRound(ctx context.Context) (*Result, error) {
	round := e.prepareProbeRound(ctx)
	if round.err != nil {
		return nil, round.err
	}
	if err := e.sendProbeSweep(ctx, round); err != nil {
		return nil, err
	}
	e.waitForProbeReplies(ctx)
	return e.buildProbeRoundResult(round.beginHop, e.finalizeProbeRound(round.effectiveMax)), nil
}

type mtrProbeRoundState struct {
	beginHop     int
	effectiveMax int
	roundID      uint32
	probeDelay   time.Duration
	err          error
}

func (e *mtrICMPEngine) prepareProbeRound(ctx context.Context) mtrProbeRoundState {
	maxHops := e.config.MaxHops
	beginHop := e.config.BeginHop
	if beginHop <= 0 {
		beginHop = 1
	}
	curRound := e.roundID.Add(1)
	e.roundFinalTTL.Store(-1)
	e.resetProbeRoundMaps()
	e.drainNotifySignal()

	probeCount := maxHops - beginHop + 1
	if probeCount < 1 {
		probeCount = 1
	}
	if err := e.rotateProbeEngineIfNeeded(ctx, probeCount); err != nil {
		return mtrProbeRoundState{err: fmt.Errorf("echoID rotation: %w", err)}
	}

	effectiveMax := e.effectiveProbeRoundMax(maxHops)
	e.initProbeRoundPreview(beginHop, effectiveMax)

	return mtrProbeRoundState{
		beginHop:     beginHop,
		effectiveMax: effectiveMax,
		roundID:      curRound,
		probeDelay:   e.probeRoundDelay(),
	}
}

func (e *mtrICMPEngine) resetProbeRoundMaps() {
	e.mu.Lock()
	e.sentAt = make(map[int]mtrProbeMeta)
	e.replied = make(map[int]*mtrProbeReply)
	e.mu.Unlock()
}

func (e *mtrICMPEngine) drainNotifySignal() {
	select {
	case <-e.notifyCh:
	default:
	}
}

func (e *mtrICMPEngine) rotateProbeEngineIfNeeded(ctx context.Context, probeCount int) error {
	if !seqWillWrap(e.seqCounter.Load(), probeCount) {
		return nil
	}
	return e.rotateEngine(ctx)
}

func (e *mtrICMPEngine) effectiveProbeRoundMax(maxHops int) int {
	if kf := e.knownFinalTTL.Load(); kf > 0 && int(kf) < maxHops {
		return int(kf)
	}
	return maxHops
}

func (e *mtrICMPEngine) initProbeRoundPreview(beginHop, effectiveMax int) {
	e.mu.Lock()
	e.curTtlSeq = make(map[int]int, effectiveMax-beginHop+1)
	e.curBeginHop = beginHop
	e.curEffectiveMax = effectiveMax
	e.mu.Unlock()
}

func (e *mtrICMPEngine) probeRoundDelay() time.Duration {
	probeDelay := time.Millisecond * time.Duration(e.config.PacketInterval)
	if probeDelay <= 0 {
		return 5 * time.Millisecond
	}
	return probeDelay
}

func (e *mtrICMPEngine) sendProbeSweep(ctx context.Context, round mtrProbeRoundState) error {
	for ttl := round.beginHop; ttl <= round.effectiveMax; ttl++ {
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}
		sent, err := e.sendProbeForTTL(ctx, ttl, round.roundID)
		if err != nil {
			return err
		}
		if !sent {
			continue
		}
		if err := e.waitProbeInterval(ctx, round.probeDelay); err != nil {
			return err
		}
	}
	return nil
}

func (e *mtrICMPEngine) sendProbeForTTL(ctx context.Context, ttl int, roundID uint32) (bool, error) {
	seq := int(e.seqCounter.Add(1) & 0xFFFF)

	// Pre-register the seq so onICMP can match it even for very short RTT replies.
	preStart := time.Now()
	e.mu.Lock()
	e.sentAt[seq] = mtrProbeMeta{ttl: ttl, start: preStart, roundID: roundID}
	e.curTtlSeq[ttl] = seq
	e.mu.Unlock()

	start, err := e.sendProbe(ctx, ttl, seq)
	if err != nil {
		// Roll back the pre-registered state on send failure.
		e.mu.Lock()
		delete(e.sentAt, seq)
		if e.curTtlSeq[ttl] == seq {
			delete(e.curTtlSeq, ttl)
		}
		e.mu.Unlock()
		if ctx.Err() != nil {
			return false, context.Cause(ctx)
		}
		var setup *probeSetupError
		if errors.As(err, &setup) {
			return false, err
		}
		return false, nil
	}

	// Update the start timestamp to the actual send time for accurate RTT.
	e.mu.Lock()
	if meta, ok := e.sentAt[seq]; ok {
		meta.start = start
		e.sentAt[seq] = meta
	}
	e.mu.Unlock()
	return true, nil
}

func (e *mtrICMPEngine) waitProbeInterval(ctx context.Context, delay time.Duration) error {
	return waitMTRTimer(ctx, delay)
}

func (e *mtrICMPEngine) waitForProbeReplies(ctx context.Context) {
	timer := time.NewTimer(e.probeResponseTimeout())
	defer timer.Stop()
	for e.hasPendingProbeReplies() {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			return
		case <-e.notifyCh:
		}
	}
}

func (e *mtrICMPEngine) probeResponseTimeout() time.Duration {
	timeout := e.config.Timeout
	if timeout <= 0 {
		return 2 * time.Second
	}
	return timeout
}

func (e *mtrICMPEngine) hasPendingProbeReplies() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.sentAt) > 0
}

func (e *mtrICMPEngine) finalizeProbeRound(effectiveMax int) int {
	e.updateKnownFinalTTLFromRound()
	return e.roundFinalMax(effectiveMax)
}

func (e *mtrICMPEngine) updateKnownFinalTTLFromRound() {
	if rf := e.roundFinalTTL.Load(); rf > 0 {
		kf := e.knownFinalTTL.Load()
		if kf < 0 || rf < kf {
			e.knownFinalTTL.Store(rf)
		}
	}
}

func (e *mtrICMPEngine) roundFinalMax(effectiveMax int) int {
	if rf := e.roundFinalTTL.Load(); rf > 0 && int(rf) < effectiveMax {
		return int(rf)
	}
	return effectiveMax
}

func (e *mtrICMPEngine) buildProbeRoundResult(beginHop, finalMax int) *Result {
	res := &Result{Hops: make([][]Hop, finalMax), responses: make(map[int][]probeResponse)}
	e.mu.Lock()
	defer e.mu.Unlock()
	for ttl := beginHop; ttl <= finalMax; ttl++ {
		res.Hops[ttl-1] = []Hop{e.probeRoundHop(ttl)}
		seq, sent := e.curTtlSeq[ttl]
		if !sent {
			continue
		}
		reply := e.replied[seq]
		if reply == nil || reply.response == nil {
			continue
		}
		res.responses[ttl] = []probeResponse{probeResponseFromMTR(reply.response)}
		if res.StopReason == nil && (reply.response.Kind == MTRResponseDestination || reply.response.Kind == MTRResponseUnreachable) {
			reason := StopReasonDestination
			if reply.response.Kind == MTRResponseUnreachable {
				reason = StopReasonUnreachable
			}
			res.StopReason = stopReasonFromMTRResponse(ttl, reason, reply.response)
		}
	}
	return res
}

func (e *mtrICMPEngine) probeRoundHop(ttl int) Hop {
	if seq, sent := e.curTtlSeq[ttl]; sent {
		if reply, ok := e.replied[seq]; ok {
			return Hop{
				Success: true,
				Address: reply.peer,
				TTL:     ttl,
				RTT:     reply.rtt,
				MPLS:    reply.mpls,
			}
		}
	}
	return Hop{
		Success: false,
		Address: nil,
		TTL:     ttl,
		RTT:     0,
		Error:   errHopLimitTimeout,
	}
}

// ---------------------------------------------------------------------------
// TCP/UDP 回退 prober：每轮调用 Traceroute + 指数退避
// ---------------------------------------------------------------------------

type mtrFallbackProber struct {
	method Method
	config Config
}

func (p *mtrFallbackProber) probeRound(ctx context.Context) (*Result, error) {
	return TracerouteWithContext(ctx, p.method, p.config)
}

func (p *mtrFallbackProber) close() {}

// ---------------------------------------------------------------------------
// ProbeTTL — single-TTL probing for per-hop scheduler
// ---------------------------------------------------------------------------

// ProbeTTL sends one ICMP echo at the given TTL and blocks until a response
// arrives, the timeout elapses, or ctx is cancelled.
func (e *mtrICMPEngine) ProbeTTL(ctx context.Context, ttl int) (mtrProbeResult, error) {
	// Exclude sequence-wrap rotation until registration, sending and bookkeeping
	// finish. Close stays independent so cancellation can interrupt socket I/O.
	e.sendMu.Lock()
	if seqWillWrap(e.seqCounter.Load(), 1) {
		if err := e.rotateEngine(ctx); err != nil {
			e.sendMu.Unlock()
			return mtrProbeResult{TTL: ttl}, fmt.Errorf("echoID rotation: %w", err)
		}
	}
	seq := int(e.seqCounter.Add(1) & 0xFFFF)

	curRound := e.roundID.Load()
	done := make(chan struct{})

	// Pre-register before sending so onICMP can match even very short RTT replies.
	preStart := time.Now()
	e.mu.Lock()
	e.sentAt[seq] = mtrProbeMeta{ttl: ttl, start: preStart, roundID: curRound}
	e.probeNotify[seq] = done
	e.mu.Unlock()

	sendStart, err := e.sendProbe(ctx, ttl, seq)
	if err != nil {
		// Roll back pre-registered state.
		e.mu.Lock()
		delete(e.sentAt, seq)
		e.closeProbeNotifyLocked(seq)
		e.mu.Unlock()
		e.sendMu.Unlock()
		if ctx.Err() != nil {
			return mtrProbeResult{TTL: ttl}, context.Cause(ctx)
		}
		// Send failed: treat as timeout (no response for this TTL)
		var setup *probeSetupError
		if errors.As(err, &setup) {
			return mtrProbeResult{TTL: ttl}, err
		}
		return mtrProbeResult{TTL: ttl}, nil
	}

	// Update to actual send timestamp for accurate RTT.
	e.mu.Lock()
	if meta, ok := e.sentAt[seq]; ok {
		meta.start = sendStart
		e.sentAt[seq] = meta
	}
	e.mu.Unlock()
	e.sendMu.Unlock()

	timeout := e.config.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-done:
		e.mu.Lock()
		reply, ok := e.replied[seq]
		if ok {
			delete(e.replied, seq)
		}
		e.mu.Unlock()

		if ok && reply != nil {
			return mtrProbeResult{
				TTL:      ttl,
				Success:  true,
				Addr:     reply.peer,
				RTT:      reply.rtt,
				MPLS:     reply.mpls,
				Response: cloneMTRProbeResponse(reply.response),
			}, nil
		}
		// Notified but no reply → was discarded (stale/bad RTT)
		return mtrProbeResult{TTL: ttl}, nil

	case <-timer.C:
		e.mu.Lock()
		delete(e.sentAt, seq)
		delete(e.probeNotify, seq)
		e.mu.Unlock()
		return mtrProbeResult{TTL: ttl}, nil

	case <-ctx.Done():
		e.mu.Lock()
		delete(e.sentAt, seq)
		delete(e.probeNotify, seq)
		e.mu.Unlock()
		return mtrProbeResult{TTL: ttl}, context.Cause(ctx)
	}
}

// Reset invalidates all in-flight probes (bumps roundID) and clears
// knownFinalTTL. In-flight ProbeTTL calls get notified and return immediately.
func (e *mtrICMPEngine) Reset() error {
	e.resetFinalTTL()
	e.roundID.Add(1)

	e.mu.Lock()
	for seq, ch := range e.probeNotify {
		close(ch)
		delete(e.probeNotify, seq)
	}
	e.sentAt = make(map[int]mtrProbeMeta)
	e.replied = make(map[int]*mtrProbeReply)
	e.mu.Unlock()

	return nil
}

// Close releases the underlying ICMP socket.
func (e *mtrICMPEngine) Close() error {
	e.close()
	return nil
}

// ---------------------------------------------------------------------------
// TCP/UDP 回退 TTL prober：单 TTL 探测
// ---------------------------------------------------------------------------

// mtrFallbackTTLProber uses Traceroute for single-TTL probing (TCP/UDP fallback).
type mtrFallbackTTLProber struct {
	method        Method
	config        Config
	asyncMetadata bool
}

func (p *mtrFallbackTTLProber) ProbeTTL(ctx context.Context, ttl int) (mtrProbeResult, error) {
	cfg := p.config
	cfg.BeginHop = ttl
	cfg.MaxHops = ttl
	cfg.NumMeasurements = 1
	cfg.MaxAttempts = 1
	cfg.ParallelRequests = 1
	cfg.RealtimePrinter = nil
	cfg.AsyncPrinter = nil
	if p.asyncMetadata {
		cfg.IPGeoSource = nil
		cfg.RDNS = false
		cfg.AlwaysWaitRDNS = false
	}

	res, err := TracerouteWithContext(ctx, p.method, cfg)
	if err != nil {
		return mtrProbeResult{TTL: ttl}, err
	}

	idx := ttl - 1
	if idx < 0 || idx >= len(res.Hops) || len(res.Hops[idx]) == 0 {
		return mtrProbeResult{TTL: ttl}, nil
	}

	h := res.Hops[idx][0]
	return mtrProbeResult{
		TTL:      ttl,
		Success:  h.Success && h.Address != nil,
		Addr:     h.Address,
		RTT:      h.RTT,
		MPLS:     h.MPLS,
		Hostname: h.Hostname,
		Geo:      h.Geo,
		Response: bestMTRProbeResponse(res, ttl),
	}, nil
}

func (p *mtrFallbackTTLProber) Reset() error { return nil }
func (p *mtrFallbackTTLProber) Close() error { return nil }
