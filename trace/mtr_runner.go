package trace

import (
	"context"
	"fmt"
	"math/rand"
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
	Interval time.Duration
	// MaxRounds 最大轮次，0 表示无限运行直到取消。
	MaxRounds int
	// IsPaused 可选：返回 true 时暂停探测（轮询检查）。
	IsPaused func() bool
}

// MTROnSnapshot 每轮完成后的回调，用于刷新 CLI 表格。
// iteration 是当前轮次（从 1 开始），stats 是截至当前的聚合快照。
type MTROnSnapshot func(iteration int, stats []MTRHopStat)

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

// RunMTR 启动 MTR 连续探测模式。
//
// ICMP 模式使用持久 raw socket（mtr 风格长驻探测，socket 只创建一次，
// 跨轮复用）。TCP/UDP 模式以 per-round Traceroute 作为回退，配合指数退避。
//
// 停止条件：ctx 取消 或 达到 MaxRounds（>0 时）。
func RunMTR(ctx context.Context, method Method, baseConfig Config, opts MTROptions, onSnapshot MTROnSnapshot) error {
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
		if err := engine.start(ctx); err != nil {
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
	defer prober.close()

	if bo == nil {
		bo = &defaultBackoff
	}

	iteration := 0
	consecutiveErrors := 0
	backoff := bo.Initial

	for {
		// ── 优先检测取消 ──
		select {
		case <-ctx.Done():
			if onSnapshot != nil {
				onSnapshot(iteration, agg.Snapshot())
			}
			return ctx.Err()
		default:
		}

		// ── 暂停等待 ──
		if opts.IsPaused != nil {
			for opts.IsPaused() {
				select {
				case <-ctx.Done():
					if onSnapshot != nil {
						onSnapshot(iteration, agg.Snapshot())
					}
					return ctx.Err()
				case <-time.After(200 * time.Millisecond):
				}
			}
		}

		// ── 执行一轮探测 ──
		res, err := prober.probeRound(ctx)

		if err != nil {
			if ctx.Err() != nil {
				if onSnapshot != nil {
					onSnapshot(iteration, agg.Snapshot())
				}
				return ctx.Err()
			}
			consecutiveErrors++
			fmt.Fprintf(os.Stderr, "mtr: probe error (%d/%d): %v\n",
				consecutiveErrors, bo.MaxConsec, err)
			if consecutiveErrors >= bo.MaxConsec {
				return fmt.Errorf("mtr: too many consecutive errors (%d), last: %w",
					consecutiveErrors, err)
			}
			select {
			case <-ctx.Done():
				if onSnapshot != nil {
					onSnapshot(iteration, agg.Snapshot())
				}
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > bo.Max {
				backoff = bo.Max
			}
			continue
		}

		// ── 成功：填充 Geo/RDNS、更新聚合器 ──
		if fillGeo {
			mtrFillGeoRDNS(res, config)
		}

		consecutiveErrors = 0
		backoff = bo.Initial

		iteration++
		stats := agg.Update(res, 1)
		if onSnapshot != nil {
			onSnapshot(iteration, stats)
		}

		if opts.MaxRounds > 0 && iteration >= opts.MaxRounds {
			return nil
		}

		// ── 等待间隔或取消 ──
		select {
		case <-ctx.Done():
			if onSnapshot != nil {
				onSnapshot(iteration, agg.Snapshot())
			}
			return ctx.Err()
		case <-time.After(opts.Interval):
		}
	}
}

// mtrFillGeoRDNS 并发查询 Result 中各 hop 的地理信息与反向 DNS。
// fetchIPData 内部有 singleflight + geoCache，重复 IP 不会重复查询。
func mtrFillGeoRDNS(res *Result, config Config) {
	var wg sync.WaitGroup
	for idx := range res.Hops {
		for j := range res.Hops[idx] {
			h := &res.Hops[idx][j]
			if !h.Success || h.Address == nil {
				continue
			}
			h.Lang = config.Lang
			wg.Add(1)
			go func(hop *Hop) {
				defer wg.Done()
				_ = hop.fetchIPData(config)
			}(h)
		}
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// 持久 ICMP 引擎（mtr 风格：raw socket 只创建一次，跨轮复用）
// ---------------------------------------------------------------------------

type mtrICMPEngine struct {
	config Config
	spec   *internal.ICMPSpec
	echoID int
	srcIP  net.IP
	ipVer  int

	// 单调递增序列号，避免跨轮 seq 冲突
	seqCounter uint32

	// per-round 探针/响应匹配
	mu       sync.Mutex
	sentAt   map[int]mtrProbeMeta // seq → probe metadata
	replied  map[int]*mtrProbeReply
	notifyCh chan struct{}

	// 当前轮次 ID，用于丢弃过期响应
	roundID uint32
}

// mtrProbeMeta 记录已发送探针的元信息，用于响应匹配。
type mtrProbeMeta struct {
	ttl     int
	start   time.Time
	roundID uint32
}

type mtrProbeReply struct {
	peer net.Addr
	rtt  time.Duration
	mpls []string
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
	if ipVer == 6 {
		srcIP, _ = util.LocalIPPortv6(config.DstIP, srcAddr, "icmp6")
	} else {
		srcIP, _ = util.LocalIPPort(config.DstIP, srcAddr, "icmp")
	}
	if srcIP == nil {
		return nil, fmt.Errorf("cannot determine local IP for MTR ICMP")
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	echoID := (r.Intn(256) << 8) | (os.Getpid() & 0xFF)

	return &mtrICMPEngine{
		config: config,
		ipVer:  ipVer,
		echoID: echoID,
		srcIP:  srcIP,
	}, nil
}

// start 创建持久 ICMP 套接字及监听协程。ctx 生命周期控制整个引擎。
func (e *mtrICMPEngine) start(ctx context.Context) error {
	e.spec = internal.NewICMPSpec(e.ipVer, e.config.ICMPMode, e.echoID, e.srcIP, e.config.DstIP)
	e.spec.InitICMP()

	e.notifyCh = make(chan struct{}, 1)
	e.sentAt = make(map[int]mtrProbeMeta)
	e.replied = make(map[int]*mtrProbeReply)

	ready := make(chan struct{})
	go e.spec.ListenICMP(ctx, ready, e.onICMP)

	select {
	case <-ready:
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return fmt.Errorf("ICMP listener startup timeout")
	}
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (e *mtrICMPEngine) close() {
	if e.spec != nil {
		e.spec.Close()
	}
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
	e.spec.Close()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	e.echoID = (r.Intn(256) << 8) | (os.Getpid() & 0xFF)
	atomic.StoreUint32(&e.seqCounter, 0)

	e.spec = internal.NewICMPSpec(e.ipVer, e.config.ICMPMode, e.echoID, e.srcIP, e.config.DstIP)
	e.spec.InitICMP()

	e.mu.Lock()
	e.notifyCh = make(chan struct{}, 1)
	e.sentAt = make(map[int]mtrProbeMeta)
	e.replied = make(map[int]*mtrProbeReply)
	e.mu.Unlock()

	ready := make(chan struct{})
	go e.spec.ListenICMP(ctx, ready, e.onICMP)

	select {
	case <-ready:
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return fmt.Errorf("ICMP listener restart timeout on echoID rotation")
	}
	return nil
}

// onICMP 是 ListenICMP 的回调：将响应匹配到已发送的探针。
func (e *mtrICMPEngine) onICMP(msg internal.ReceivedMessage, finish time.Time, seq int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	start, ok := e.sentAt[seq]
	if !ok {
		return
	}

	// 丢弃过期轮次的响应
	if start.roundID != atomic.LoadUint32(&e.roundID) {
		delete(e.sentAt, seq)
		return
	}

	// seq 只有 16 位，长期运行会回卷。迟到回包的 seq 可能恰好
	// 与当前轮次的某个探针重合，而 roundID 检查无法区分。
	// 通过 RTT 合理性检查兜底：RTT ≤ 0 或超过探测超时的响应一律丢弃。
	rtt := finish.Sub(start.start)
	maxRTT := e.config.Timeout
	if maxRTT <= 0 {
		maxRTT = 2 * time.Second
	}
	if rtt <= 0 || rtt > maxRTT {
		delete(e.sentAt, seq)
		return
	}

	e.replied[seq] = &mtrProbeReply{
		peer: msg.Peer,
		rtt:  rtt,
		mpls: extractMPLS(msg),
	}
	delete(e.sentAt, seq)

	select {
	case e.notifyCh <- struct{}{}:
	default:
	}
}

// sendProbe 发送一个 ICMP echo（IPv4 或 IPv6），返回发送时间戳。
func (e *mtrICMPEngine) sendProbe(ctx context.Context, ttl, seq int) (time.Time, error) {
	payload := make([]byte, e.config.PktSize)
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
		}
		icmpHdr := &layers.ICMPv4{
			TypeCode: layers.CreateICMPv4TypeCode(layers.ICMPv4TypeEchoRequest, 0),
			Id:       uint16(e.echoID),
			Seq:      uint16(seq),
		}
		return e.spec.SendICMP(ctx, ipHdr, icmpHdr, nil, payload)
	}

	// IPv6
	ipHdr := &layers.IPv6{
		Version:    6,
		SrcIP:      e.srcIP,
		DstIP:      e.config.DstIP,
		NextHeader: layers.IPProtocolICMPv6,
		HopLimit:   uint8(ttl),
	}
	icmpHdr := &layers.ICMPv6{
		TypeCode: layers.CreateICMPv6TypeCode(layers.ICMPv6TypeEchoRequest, 0),
	}
	_ = icmpHdr.SetNetworkLayerForChecksum(ipHdr)
	icmpEcho := &layers.ICMPv6Echo{
		Identifier: uint16(e.echoID),
		SeqNumber:  uint16(seq),
	}
	return e.spec.SendICMP(ctx, ipHdr, icmpHdr, icmpEcho, payload)
}

// probeRound 执行一轮持久探测：对每个 TTL 发送一个 ICMP echo，
// 收集响应后构造与 Traceroute 兼容的 *Result。
func (e *mtrICMPEngine) probeRound(ctx context.Context) (*Result, error) {
	maxHops := e.config.MaxHops
	beginHop := e.config.BeginHop
	if beginHop <= 0 {
		beginHop = 1
	}

	// 重置 per-round 状态，递增轮次 ID
	curRound := atomic.AddUint32(&e.roundID, 1)
	e.mu.Lock()
	e.sentAt = make(map[int]mtrProbeMeta)
	e.replied = make(map[int]*mtrProbeReply)
	e.mu.Unlock()

	// 排空 notify 通道
	select {
	case <-e.notifyCh:
	default:
	}

	// 检查 seq 回卷：如果本轮探测会导致 16 位序列号跨越 0xFFFF 边界，
	// 轮换 echoID 并重建 ICMP 引擎，确保新旧探针在协议层彻底隔离。
	probeCount := maxHops - beginHop + 1
	if seqWillWrap(atomic.LoadUint32(&e.seqCounter), probeCount) {
		if err := e.rotateEngine(ctx); err != nil {
			return nil, fmt.Errorf("echoID rotation: %w", err)
		}
	}

	// inter-probe delay：优先使用 Config.PacketInterval，默认 5ms
	probeDelay := time.Millisecond * time.Duration(e.config.PacketInterval)
	if probeDelay <= 0 {
		probeDelay = 5 * time.Millisecond
	}

	// ttl → seq 映射，用于构造 Result 时查找响应
	ttlSeq := make(map[int]int, maxHops-beginHop+1)

	// ── 逐 TTL 发送探针（mtr 风格顺序发送）──
	for ttl := beginHop; ttl <= maxHops; ttl++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		seq := int(atomic.AddUint32(&e.seqCounter, 1) & 0xFFFF)

		start, err := e.sendProbe(ctx, ttl, seq)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			// 该 TTL 发送失败，当作超时处理
			continue
		}

		e.mu.Lock()
		e.sentAt[seq] = mtrProbeMeta{ttl: ttl, start: start, roundID: curRound}
		e.mu.Unlock()
		ttlSeq[ttl] = seq

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(probeDelay):
		}
	}

	// ── 等待响应（带超时）──
	timeout := e.config.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	deadline := time.After(timeout)

waitLoop:
	for {
		e.mu.Lock()
		pending := len(e.sentAt)
		e.mu.Unlock()
		if pending == 0 {
			break
		}
		select {
		case <-ctx.Done():
			break waitLoop
		case <-deadline:
			break waitLoop
		case <-e.notifyCh:
			// 有新响应到达，继续检查
		}
	}

	// ── 构造 Result ──
	res := &Result{Hops: make([][]Hop, maxHops)}

	e.mu.Lock()
	for ttl := beginHop; ttl <= maxHops; ttl++ {
		idx := ttl - 1
		seq, sent := ttlSeq[ttl]
		if sent {
			if reply, ok := e.replied[seq]; ok {
				res.Hops[idx] = []Hop{{
					Success: true,
					Address: reply.peer,
					TTL:     ttl,
					RTT:     reply.rtt,
					MPLS:    reply.mpls,
				}}
				continue
			}
		}
		res.Hops[idx] = []Hop{{
			Success: false,
			Address: nil,
			TTL:     ttl,
			RTT:     0,
			Error:   errHopLimitTimeout,
		}}
	}
	e.mu.Unlock()

	return res, nil
}

// ---------------------------------------------------------------------------
// TCP/UDP 回退 prober：每轮调用 Traceroute + 指数退避
// ---------------------------------------------------------------------------

type mtrFallbackProber struct {
	method Method
	config Config
}

func (p *mtrFallbackProber) probeRound(_ context.Context) (*Result, error) {
	return Traceroute(p.method, p.config)
}

func (p *mtrFallbackProber) close() {}
