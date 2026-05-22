package trace

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"
)

const (
	// mtrMetadataNegativeCacheTTL 避免失败的异步 metadata 查询持续打 Geo/RDNS 服务，
	// 同时让长时间运行的 TUI 仍能较快重试补全。
	mtrMetadataNegativeCacheTTL = 3 * time.Second
	// mtrAsyncMetadataGeoConcurrency 只限制 MTR TUI 的异步 GeoIP 查询；
	// 探测并发和非 TUI 的 MTR metadata 路径由其他逻辑控制。
	mtrAsyncMetadataGeoConcurrency = 5
	// mtrAsyncMetadataHostConcurrency 单独限制反向 DNS 查询，
	// 避免慢 PTR 响应占用 GeoIP 查询槽位。
	mtrAsyncMetadataHostConcurrency = 6
)

type mtrMetadataKind uint8

const (
	mtrMetadataKindGeo mtrMetadataKind = iota + 1
	mtrMetadataKindHost
)

type mtrSchedulerRuntime struct {
	ctx                  context.Context
	metadataCtx          context.Context
	metadataCancel       context.CancelFunc
	doneCh               chan struct{}
	prober               mtrTTLProber
	agg                  *MTRAggregator
	cfg                  mtrSchedulerConfig
	onSnapshot           MTROnSnapshot
	onProbe              func(result mtrProbeResult, iteration int, at time.Time)
	beginHop             int
	maxHops              int
	parallelism          int
	hopInterval          time.Duration
	progressDelay        time.Duration
	maxConsecErrs        int
	maxInFlightHop       int
	states               []mtrHopState
	generation           uint64
	knownFinalTTL        int32
	inFlight             int
	resultCh             chan mtrCompletedProbe
	metadataCh           chan mtrMetadataResult
	metadataGeoSlots     chan struct{}
	metadataHostSlots    chan struct{}
	metadataGeoInFlight  map[string]uint64
	metadataHostInFlight map[string]uint64
	metadataCache        map[string]mtrMetadataPatch
	metadataGeoBackoff   map[string]time.Time
	metadataHostBackoff  map[string]time.Time
	lastSnapshot         time.Time
}

type mtrMetadataResult struct {
	patch mtrMetadataPatch
	kind  mtrMetadataKind
	gen   uint64
}

func newMTRSchedulerRuntime(
	ctx context.Context,
	prober mtrTTLProber,
	agg *MTRAggregator,
	cfg mtrSchedulerConfig,
	onSnapshot MTROnSnapshot,
	onProbe func(result mtrProbeResult, iteration int, at time.Time),
) (*mtrSchedulerRuntime, error) {
	beginHop := cfg.BeginHop
	if beginHop <= 0 {
		beginHop = 1
	}

	maxHops := cfg.MaxHops
	if maxHops <= 0 {
		maxHops = 30
	}
	if maxHops > 255 {
		maxHops = 255
	}
	if beginHop > maxHops {
		return nil, fmt.Errorf("mtr: beginHop (%d) > maxHops (%d)", beginHop, maxHops)
	}

	parallelism := cfg.ParallelRequests
	if parallelism < 1 {
		parallelism = 1
	}

	hopInterval := cfg.HopInterval
	if hopInterval <= 0 {
		hopInterval = time.Second
	}

	progressDelay := cfg.ProgressThrottle
	if progressDelay <= 0 {
		progressDelay = 200 * time.Millisecond
	}

	maxConsecErrs := cfg.MaxConsecErrors
	if maxConsecErrs <= 0 {
		maxConsecErrs = 10
	}

	maxInFlightHop := cfg.MaxInFlightPerHop
	if maxInFlightHop <= 0 {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 2 * time.Second
		}
		maxInFlightHop = int((timeout+hopInterval-1)/hopInterval) + 1
		if maxInFlightHop < 1 {
			maxInFlightHop = 1
		}
	}

	metadataCtx, metadataCancel := context.WithCancel(ctx)

	return &mtrSchedulerRuntime{
		ctx:                  ctx,
		metadataCtx:          metadataCtx,
		metadataCancel:       metadataCancel,
		doneCh:               make(chan struct{}),
		prober:               prober,
		agg:                  agg,
		cfg:                  cfg,
		onSnapshot:           onSnapshot,
		onProbe:              onProbe,
		beginHop:             beginHop,
		maxHops:              maxHops,
		parallelism:          parallelism,
		hopInterval:          hopInterval,
		progressDelay:        progressDelay,
		maxConsecErrs:        maxConsecErrs,
		maxInFlightHop:       maxInFlightHop,
		states:               make([]mtrHopState, maxHops+1),
		knownFinalTTL:        -1,
		resultCh:             make(chan mtrCompletedProbe, parallelism*2),
		metadataCh:           make(chan mtrMetadataResult, parallelism*2),
		metadataGeoSlots:     make(chan struct{}, mtrAsyncMetadataGeoConcurrency),
		metadataHostSlots:    make(chan struct{}, mtrAsyncMetadataHostConcurrency),
		metadataGeoInFlight:  make(map[string]uint64),
		metadataHostInFlight: make(map[string]uint64),
		metadataCache:        make(map[string]mtrMetadataPatch),
		metadataGeoBackoff:   make(map[string]time.Time),
		metadataHostBackoff:  make(map[string]time.Time),
	}, nil
}

func (rt *mtrSchedulerRuntime) run() error {
	defer close(rt.doneCh)
	defer rt.cancelMetadataLookups()

	rt.scheduleReady()

	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-rt.ctx.Done():
			return rt.handleCancel()
		case cp := <-rt.resultCh:
			rt.processResult(cp)
			if rt.isDone() {
				rt.maybeSnapshot(true)
				return nil
			}
			rt.scheduleReady()
		case mr := <-rt.metadataCh:
			rt.processMetadataResult(mr)
			if rt.isDone() {
				rt.maybeSnapshot(true)
				return nil
			}
		case <-tick.C:
			rt.handleReset()
			rt.scheduleReady()
			if rt.isDone() {
				rt.maybeSnapshot(true)
				return nil
			}
		}
	}
}

func (rt *mtrSchedulerRuntime) effectiveMax() int {
	kf := atomic.LoadInt32(&rt.knownFinalTTL)
	if kf > 0 && int(kf) < rt.maxHops {
		return int(kf)
	}
	return rt.maxHops
}

func (rt *mtrSchedulerRuntime) computeIteration() int {
	eMax := rt.effectiveMax()
	minSnt := -1
	for ttl := rt.beginHop; ttl <= eMax; ttl++ {
		if rt.states[ttl].disabled {
			continue
		}
		snt := rt.states[ttl].completed
		if minSnt < 0 || snt < minSnt {
			minSnt = snt
		}
	}
	if minSnt < 0 {
		return 0
	}
	return minSnt
}

func (rt *mtrSchedulerRuntime) maybeSnapshot(force bool) {
	if rt.onSnapshot == nil {
		return
	}
	now := time.Now()
	if !force && now.Sub(rt.lastSnapshot) < rt.progressDelay {
		return
	}
	rt.lastSnapshot = now
	rt.onSnapshot(rt.computeIteration(), rt.agg.Snapshot())
}

func (rt *mtrSchedulerRuntime) launchProbe(ttl int) {
	rt.states[ttl].inFlightCount++
	rt.states[ttl].nextAt = time.Now().Add(rt.hopInterval)
	rt.inFlight++

	gen := rt.generation
	go func() {
		result, err := rt.prober.ProbeTTL(rt.ctx, ttl)
		rt.resultCh <- mtrCompletedProbe{
			ttl:    ttl,
			result: result,
			gen:    gen,
			doneAt: time.Now(),
			err:    err,
		}
	}()
}

func (rt *mtrSchedulerRuntime) processResult(cp mtrCompletedProbe) {
	rt.inFlight--
	if cp.gen != rt.generation {
		return
	}
	if cp.ttl < rt.beginHop || cp.ttl > rt.maxHops {
		return
	}

	state := &rt.states[cp.ttl]
	state.inFlightCount--
	if state.disabled {
		return
	}
	if cp.err != nil {
		rt.processProbeError(cp.ttl, cp.err, cp.doneAt)
		return
	}
	rt.processProbeSuccess(cp.ttl, cp.result, cp.doneAt)
}

func (rt *mtrSchedulerRuntime) processProbeError(ttl int, err error, doneAt time.Time) {
	if rt.ctx.Err() != nil {
		return
	}

	state := &rt.states[ttl]
	state.consecutiveErrs++
	fmt.Fprintf(os.Stderr, "mtr: probe error (%d/%d): %v\n", state.consecutiveErrs, rt.maxConsecErrs, err)
	if state.consecutiveErrs < rt.maxConsecErrs {
		return
	}

	state.consecutiveErrs = 0
	state.completed++
	rt.recordSyntheticTimeout(ttl, doneAt)
}

func (rt *mtrSchedulerRuntime) recordSyntheticTimeout(ttl int, doneAt time.Time) {
	rt.agg.Update(rt.timeoutProbeResult(ttl), 1)
	if rt.onProbe != nil {
		rt.onProbe(mtrProbeResult{TTL: ttl}, rt.computeIteration(), doneAt)
	}
	rt.maybeSnapshot(false)
}

func (rt *mtrSchedulerRuntime) resultHopCount() int {
	if n := len(rt.states) - 1; n > 0 {
		return n
	}
	if rt.maxHops > 0 {
		return rt.maxHops
	}
	return 0
}

func (rt *mtrSchedulerRuntime) timeoutProbeResult(ttl int) *Result {
	singleRes := &Result{Hops: make([][]Hop, rt.resultHopCount())}
	idx := ttl - 1
	if idx >= 0 && idx < len(singleRes.Hops) {
		singleRes.Hops[idx] = []Hop{{TTL: ttl, Error: errHopLimitTimeout}}
	}
	return singleRes
}

func (rt *mtrSchedulerRuntime) processProbeSuccess(ttl int, result mtrProbeResult, doneAt time.Time) {
	rt.detectDestination(ttl, result)
	if rt.probeBudgetReached(ttl) {
		rt.states[ttl].consecutiveErrs = 0
		return
	}

	rt.markProbeCompleted(ttl)
	result = rt.applyMetadataCache(result)
	singleRes := rt.singleProbeResult(ttl, result)
	if rt.shouldFetchMetadataAsync(result) {
		rt.maybeLaunchMetadataLookup(result)
	} else if rt.cfg.FillGeo && result.Geo == nil {
		mtrFillGeoRDNS(singleRes, rt.cfg.BaseConfig)
	}

	rt.agg.Update(singleRes, 1)
	if rt.onProbe != nil {
		rt.onProbe(result, rt.computeIteration(), doneAt)
	}
	rt.maybeSnapshot(false)
}

func (rt *mtrSchedulerRuntime) applyMetadataCache(result mtrProbeResult) mtrProbeResult {
	ip := mtrAddrString(result.Addr)
	if ip == "" {
		return result
	}
	patch, ok := rt.metadataCache[ip]
	if !ok {
		return result
	}
	if result.Hostname == "" && patch.host != "" {
		result.Hostname = patch.host
	}
	if result.Geo == nil && patch.geo != nil {
		result.Geo = patch.geo
	}
	return result
}

func (rt *mtrSchedulerRuntime) shouldFetchMetadataAsync(result mtrProbeResult) bool {
	if !rt.cfg.FillGeo || !rt.cfg.AsyncMetadata || result.Addr == nil {
		return false
	}
	if rt.cfg.BaseConfig.IPGeoSource == nil && !rt.cfg.BaseConfig.RDNS {
		return false
	}
	needsGeo := rt.cfg.BaseConfig.IPGeoSource != nil && result.Geo == nil
	needsHost := rt.cfg.BaseConfig.RDNS && result.Hostname == ""
	return needsGeo || needsHost
}

func (rt *mtrSchedulerRuntime) maybeLaunchMetadataLookup(result mtrProbeResult) {
	ip := mtrAddrString(result.Addr)
	if ip == "" {
		return
	}

	gen := rt.generation
	cfg := rt.cfg.BaseConfig
	generationCtx := rt.metadataCtx
	if generationCtx == nil {
		generationCtx = rt.ctx
	}

	rt.maybeLaunchGeoMetadataLookup(ip, result, cfg, generationCtx, gen, false)
	rt.maybeLaunchHostMetadataLookup(ip, result, cfg, generationCtx, gen)
}

func (rt *mtrSchedulerRuntime) maybeLaunchGeoMetadataLookup(
	ip string,
	result mtrProbeResult,
	cfg Config,
	generationCtx context.Context,
	gen uint64,
	allowDN42IPOnly bool,
) {
	if cfg.IPGeoSource == nil || result.Geo != nil {
		return
	}
	if _, ok := rt.metadataGeoInFlight[ip]; ok {
		return
	}
	if rt.metadataGeoBackoffActive(ip, time.Now()) {
		return
	}

	host := result.Hostname
	if host == "" {
		host = rt.metadataCache[ip].host
	}
	// DN42 的 Geo 查询会把 PTR 拼进 "ip,host"。RDNS 开启但 host 还没出来时，
	// 先等 Host lookup 结束，避免异步拆分后把纯 IP 结果写入缓存。
	if cfg.DN42 && cfg.RDNS && host == "" && !allowDN42IPOnly {
		return
	}
	rt.metadataGeoInFlight[ip] = gen
	go rt.runMetadataLookup(generationCtx, gen, mtrMetadataKindGeo, rt.metadataGeoSlots, func(cfg Config) mtrMetadataPatch {
		return lookupMTRGeoMetadata(result.Addr, cfg, host)
	}, cfg)
}

func (rt *mtrSchedulerRuntime) maybeLaunchHostMetadataLookup(ip string, result mtrProbeResult, cfg Config, generationCtx context.Context, gen uint64) {
	if !cfg.RDNS || result.Hostname != "" {
		return
	}
	if _, ok := rt.metadataHostInFlight[ip]; ok {
		return
	}
	if rt.metadataHostBackoffActive(ip, time.Now()) {
		return
	}

	rt.metadataHostInFlight[ip] = gen
	go rt.runMetadataLookup(generationCtx, gen, mtrMetadataKindHost, rt.metadataHostSlots, func(cfg Config) mtrMetadataPatch {
		return lookupMTRHostMetadata(result.Addr, cfg)
	}, cfg)
}

func (rt *mtrSchedulerRuntime) runMetadataLookup(
	generationCtx context.Context,
	gen uint64,
	kind mtrMetadataKind,
	slots chan struct{},
	lookup func(Config) mtrMetadataPatch,
	cfg Config,
) {
	if !rt.acquireMetadataSlot(generationCtx, slots) {
		return
	}
	defer rt.releaseMetadataSlot(slots)

	lookupCtx, cancel := context.WithTimeout(generationCtx, mtrMetadataLookupTimeout(cfg.Timeout))
	defer cancel()
	cfg.Context = lookupCtx
	patch := lookup(cfg)
	if generationCtx.Err() != nil {
		return
	}
	select {
	case rt.metadataCh <- mtrMetadataResult{patch: patch, kind: kind, gen: gen}:
	case <-generationCtx.Done():
	case <-rt.doneCh:
	}
}

func (rt *mtrSchedulerRuntime) acquireMetadataSlot(ctx context.Context, slots chan struct{}) bool {
	select {
	case slots <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	case <-rt.doneCh:
		return false
	}
}

func (rt *mtrSchedulerRuntime) releaseMetadataSlot(slots chan struct{}) {
	select {
	case <-slots:
	default:
	}
}

func mtrMetadataLookupTimeout(probeTimeout time.Duration) time.Duration {
	floor := geoTimeoutForAttempt(0)
	if probeTimeout > floor {
		return probeTimeout
	}
	return floor
}

func (rt *mtrSchedulerRuntime) processMetadataResult(result mtrMetadataResult) {
	if result.gen != rt.generation {
		return
	}

	ip := result.patch.ip
	if ip == "" {
		return
	}
	rt.clearMetadataInFlight(result.kind, ip)
	rt.cacheMetadataPatch(result.kind, result.patch)
	rt.maybeLaunchDN42GeoAfterHostResult(result)
	if !rt.agg.PatchMetadataByIP(ip, result.patch.host, result.patch.geo) {
		return
	}
	rt.maybeSnapshot(false)
}

func (rt *mtrSchedulerRuntime) maybeLaunchDN42GeoAfterHostResult(result mtrMetadataResult) {
	if result.kind != mtrMetadataKindHost {
		return
	}
	cfg := rt.cfg.BaseConfig
	if !cfg.DN42 || !cfg.RDNS || cfg.IPGeoSource == nil {
		return
	}
	ip := result.patch.ip
	cached := rt.metadataCache[ip]
	if ip == "" || cached.geo != nil {
		return
	}
	addrIP := net.ParseIP(ip)
	if addrIP == nil {
		return
	}
	generationCtx := rt.metadataCtx
	if generationCtx == nil {
		generationCtx = rt.ctx
	}
	// Host lookup 结束后再允许 IP-only fallback：PTR 为空时仍补 Geo，
	// PTR 成功时则用缓存的 host 发起 "ip,host" 查询。
	rt.maybeLaunchGeoMetadataLookup(ip, mtrProbeResult{
		Addr:     &net.IPAddr{IP: addrIP},
		Hostname: cached.host,
		Geo:      cached.geo,
	}, cfg, generationCtx, result.gen, true)
}

func (rt *mtrSchedulerRuntime) clearMetadataInFlight(kind mtrMetadataKind, ip string) {
	switch kind {
	case mtrMetadataKindGeo:
		delete(rt.metadataGeoInFlight, ip)
	case mtrMetadataKindHost:
		delete(rt.metadataHostInFlight, ip)
	}
}

func (rt *mtrSchedulerRuntime) cacheMetadataPatch(kind mtrMetadataKind, patch mtrMetadataPatch) {
	if patch.ip == "" {
		return
	}
	now := time.Now()
	cached := rt.metadataCache[patch.ip]
	if cached.ip == "" {
		cached.ip = patch.ip
	}

	switch kind {
	case mtrMetadataKindGeo:
		if patch.geo == nil {
			rt.metadataGeoBackoff[patch.ip] = now.Add(mtrMetadataNegativeCacheTTL)
		} else {
			delete(rt.metadataGeoBackoff, patch.ip)
			if cached.geo == nil {
				cached.geo = patch.geo
			}
		}
	case mtrMetadataKindHost:
		if patch.host == "" {
			rt.metadataHostBackoff[patch.ip] = now.Add(mtrMetadataNegativeCacheTTL)
		} else {
			delete(rt.metadataHostBackoff, patch.ip)
			if cached.host == "" {
				cached.host = patch.host
			}
		}
	}
	rt.metadataCache[patch.ip] = cached
}

func (rt *mtrSchedulerRuntime) metadataGeoBackoffActive(ip string, now time.Time) bool {
	return rt.metadataBackoffActive(rt.metadataGeoBackoff, ip, now)
}

func (rt *mtrSchedulerRuntime) metadataHostBackoffActive(ip string, now time.Time) bool {
	return rt.metadataBackoffActive(rt.metadataHostBackoff, ip, now)
}

func (rt *mtrSchedulerRuntime) metadataBackoffActive(backoff map[string]time.Time, ip string, now time.Time) bool {
	until, ok := backoff[ip]
	if !ok {
		return false
	}
	if now.Before(until) {
		return true
	}
	delete(backoff, ip)
	return false
}

func (rt *mtrSchedulerRuntime) detectDestination(ttl int, result mtrProbeResult) {
	if !result.Success || result.Addr == nil {
		return
	}

	peerIP := mtrAddrToIP(result.Addr)
	if peerIP == nil || !peerIP.Equal(rt.cfg.DstIP) {
		return
	}

	curFinal := atomic.LoadInt32(&rt.knownFinalTTL)
	if curFinal < 0 {
		atomic.StoreInt32(&rt.knownFinalTTL, int32(ttl))
		rt.disableHigherTTLs(ttl + 1)
		return
	}

	if int32(ttl) < curFinal {
		oldFinal := int(curFinal)
		atomic.StoreInt32(&rt.knownFinalTTL, int32(ttl))
		rt.disableHigherTTLs(ttl + 1)
		rt.agg.ClearHop(oldFinal)
	}
}

func (rt *mtrSchedulerRuntime) disableHigherTTLs(fromTTL int) {
	for ttl := fromTTL; ttl <= rt.maxHops; ttl++ {
		rt.states[ttl].disabled = true
	}
}

func (rt *mtrSchedulerRuntime) probeBudgetReached(ttl int) bool {
	return rt.cfg.MaxPerHop > 0 && rt.states[ttl].completed >= rt.cfg.MaxPerHop
}

func (rt *mtrSchedulerRuntime) markProbeCompleted(ttl int) {
	rt.states[ttl].consecutiveErrs = 0
	rt.states[ttl].completed++
}

func (rt *mtrSchedulerRuntime) singleProbeResult(ttl int, result mtrProbeResult) *Result {
	singleRes := &Result{Hops: make([][]Hop, rt.resultHopCount())}
	hop := Hop{
		Success:  result.Success,
		Address:  result.Addr,
		Hostname: result.Hostname,
		TTL:      ttl,
		RTT:      result.RTT,
		MPLS:     result.MPLS,
		Geo:      result.Geo,
		Lang:     rt.cfg.BaseConfig.Lang,
	}
	if !hop.Success && hop.Address == nil {
		hop.Error = errHopLimitTimeout
	}

	idx := ttl - 1
	if idx >= 0 && idx < len(singleRes.Hops) {
		singleRes.Hops[idx] = []Hop{hop}
	}
	return singleRes
}

func (rt *mtrSchedulerRuntime) scheduleReady() {
	if rt.cfg.IsPaused != nil && rt.cfg.IsPaused() {
		return
	}

	now := time.Now()
	eMax := rt.effectiveMax()
	for ttl := rt.beginHop; ttl <= eMax; ttl++ {
		if rt.inFlight >= rt.parallelism {
			return
		}
		if rt.canLaunchProbe(ttl, now) {
			rt.launchProbe(ttl)
		}
	}
}

func (rt *mtrSchedulerRuntime) canLaunchProbe(ttl int, now time.Time) bool {
	state := &rt.states[ttl]
	if state.disabled || state.inFlightCount >= rt.maxInFlightHop {
		return false
	}
	if rt.cfg.MaxPerHop > 0 && state.completed+state.inFlightCount >= rt.cfg.MaxPerHop {
		return false
	}
	if !state.nextAt.IsZero() && now.Before(state.nextAt) {
		return false
	}
	return true
}

func (rt *mtrSchedulerRuntime) isDone() bool {
	if rt.cfg.MaxPerHop <= 0 {
		return false
	}

	eMax := rt.effectiveMax()
	for ttl := rt.beginHop; ttl <= eMax; ttl++ {
		state := &rt.states[ttl]
		if state.disabled {
			continue
		}
		if state.completed < rt.cfg.MaxPerHop || state.inFlightCount > 0 {
			return false
		}
	}
	if rt.inFlight != 0 {
		return false
	}
	return len(rt.metadataGeoInFlight) == 0 && len(rt.metadataHostInFlight) == 0
}

func (rt *mtrSchedulerRuntime) handleReset() {
	if rt.cfg.IsResetRequested == nil || !rt.cfg.IsResetRequested() {
		return
	}

	rt.generation++
	rt.resetMetadataContext()
	for idx := range rt.states {
		rt.states[idx] = mtrHopState{}
	}
	clear(rt.metadataGeoInFlight)
	clear(rt.metadataHostInFlight)
	clear(rt.metadataCache)
	clear(rt.metadataGeoBackoff)
	clear(rt.metadataHostBackoff)
	atomic.StoreInt32(&rt.knownFinalTTL, -1)
	rt.agg.Reset()
	_ = rt.prober.Reset()
}

func (rt *mtrSchedulerRuntime) resetMetadataContext() {
	rt.cancelMetadataLookups()
	rt.metadataCtx, rt.metadataCancel = context.WithCancel(rt.ctx)
}

func (rt *mtrSchedulerRuntime) cancelMetadataLookups() {
	if rt.metadataCancel != nil {
		rt.metadataCancel()
	}
}

func (rt *mtrSchedulerRuntime) handleCancel() error {
	rt.drainInFlight()
	rt.maybeSnapshot(true)
	return rt.ctx.Err()
}

func (rt *mtrSchedulerRuntime) drainInFlight() {
	deadline := time.After(5 * time.Second)
	for rt.inFlight > 0 {
		select {
		case <-rt.resultCh:
			rt.inFlight--
		case <-deadline:
			return
		}
	}
}
