package trace

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

const mtrMetadataNegativeCacheTTL = 5 * time.Second

type mtrSchedulerRuntime struct {
	ctx              context.Context
	metadataCtx      context.Context
	metadataCancel   context.CancelFunc
	doneCh           chan struct{}
	prober           mtrTTLProber
	agg              *MTRAggregator
	cfg              mtrSchedulerConfig
	onSnapshot       MTROnSnapshot
	onProbe          func(result mtrProbeResult, iteration int)
	beginHop         int
	maxHops          int
	parallelism      int
	hopInterval      time.Duration
	progressDelay    time.Duration
	maxConsecErrs    int
	maxInFlightHop   int
	states           []mtrHopState
	generation       uint64
	knownFinalTTL    int32
	inFlight         int
	resultCh         chan mtrCompletedProbe
	metadataCh       chan mtrMetadataResult
	metadataInFlight map[string]uint64
	metadataCache    map[string]mtrMetadataPatch
	metadataBackoff  map[string]time.Time
	lastSnapshot     time.Time
}

type mtrMetadataResult struct {
	patch mtrMetadataPatch
	gen   uint64
}

func newMTRSchedulerRuntime(
	ctx context.Context,
	prober mtrTTLProber,
	agg *MTRAggregator,
	cfg mtrSchedulerConfig,
	onSnapshot MTROnSnapshot,
	onProbe func(result mtrProbeResult, iteration int),
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
		ctx:              ctx,
		metadataCtx:      metadataCtx,
		metadataCancel:   metadataCancel,
		doneCh:           make(chan struct{}),
		prober:           prober,
		agg:              agg,
		cfg:              cfg,
		onSnapshot:       onSnapshot,
		onProbe:          onProbe,
		beginHop:         beginHop,
		maxHops:          maxHops,
		parallelism:      parallelism,
		hopInterval:      hopInterval,
		progressDelay:    progressDelay,
		maxConsecErrs:    maxConsecErrs,
		maxInFlightHop:   maxInFlightHop,
		states:           make([]mtrHopState, maxHops+1),
		knownFinalTTL:    -1,
		resultCh:         make(chan mtrCompletedProbe, parallelism*2),
		metadataCh:       make(chan mtrMetadataResult, parallelism*2),
		metadataInFlight: make(map[string]uint64),
		metadataCache:    make(map[string]mtrMetadataPatch),
		metadataBackoff:  make(map[string]time.Time),
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
		rt.processProbeError(cp.ttl, cp.err)
		return
	}
	rt.processProbeSuccess(cp.ttl, cp.result)
}

func (rt *mtrSchedulerRuntime) processProbeError(ttl int, err error) {
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
	rt.recordSyntheticTimeout(ttl)
}

func (rt *mtrSchedulerRuntime) recordSyntheticTimeout(ttl int) {
	rt.agg.Update(rt.timeoutProbeResult(ttl), 1)
	if rt.onProbe != nil {
		rt.onProbe(mtrProbeResult{TTL: ttl}, rt.computeIteration())
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

func (rt *mtrSchedulerRuntime) processProbeSuccess(ttl int, result mtrProbeResult) {
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
		rt.onProbe(result, rt.computeIteration())
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
	if _, ok := rt.metadataInFlight[ip]; ok {
		return
	}
	if rt.metadataBackoffActive(ip, time.Now()) {
		return
	}

	gen := rt.generation
	cfg := rt.cfg.BaseConfig
	metadataTimeout := cfg.Timeout
	if metadataTimeout <= 0 {
		metadataTimeout = geoTimeoutForAttempt(0)
	}
	generationCtx := rt.metadataCtx
	if generationCtx == nil {
		generationCtx = rt.ctx
	}
	lookupCtx, cancel := context.WithTimeout(generationCtx, metadataTimeout)
	cfg.Context = lookupCtx
	addr := result.Addr
	rt.metadataInFlight[ip] = gen

	go func() {
		defer cancel()
		patch := lookupMTRMetadata(addr, cfg)
		if generationCtx.Err() != nil {
			return
		}
		select {
		case rt.metadataCh <- mtrMetadataResult{patch: patch, gen: gen}:
		case <-generationCtx.Done():
		case <-rt.doneCh:
		}
	}()
}

func (rt *mtrSchedulerRuntime) processMetadataResult(result mtrMetadataResult) {
	if result.gen != rt.generation {
		return
	}

	ip := result.patch.ip
	if ip == "" {
		return
	}
	delete(rt.metadataInFlight, ip)
	rt.cacheMetadataPatch(result.patch)
	if !rt.agg.PatchMetadataByIP(ip, result.patch.host, result.patch.geo) {
		return
	}
	rt.maybeSnapshot(false)
}

func (rt *mtrSchedulerRuntime) cacheMetadataPatch(patch mtrMetadataPatch) {
	if patch.ip == "" {
		return
	}
	if patch.host == "" && patch.geo == nil {
		rt.metadataBackoff[patch.ip] = time.Now().Add(mtrMetadataNegativeCacheTTL)
		return
	}
	delete(rt.metadataBackoff, patch.ip)

	cached := rt.metadataCache[patch.ip]
	if cached.ip == "" {
		cached.ip = patch.ip
	}
	if cached.host == "" && patch.host != "" {
		cached.host = patch.host
	}
	if cached.geo == nil && patch.geo != nil {
		cached.geo = patch.geo
	}
	rt.metadataCache[patch.ip] = cached
}

func (rt *mtrSchedulerRuntime) metadataBackoffActive(ip string, now time.Time) bool {
	until, ok := rt.metadataBackoff[ip]
	if !ok {
		return false
	}
	if now.Before(until) {
		return true
	}
	delete(rt.metadataBackoff, ip)
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
	return len(rt.metadataInFlight) == 0
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
	clear(rt.metadataInFlight)
	clear(rt.metadataCache)
	clear(rt.metadataBackoff)
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
