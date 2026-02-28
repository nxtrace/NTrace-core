package trace

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

// ---------------------------------------------------------------------------
// Per-hop independent scheduler (CLI MTR mode)
// ---------------------------------------------------------------------------

// mtrProbeResult holds the outcome of a single TTL probe.
type mtrProbeResult struct {
	TTL      int
	Success  bool
	Addr     net.Addr
	RTT      time.Duration
	MPLS     []string
	Hostname string           // pre-resolved PTR (fallback prober)
	Geo      *ipgeo.IPGeoData // pre-resolved geo  (fallback prober)
}

// mtrTTLProber abstracts single-TTL probing for the per-hop scheduler.
type mtrTTLProber interface {
	// ProbeTTL sends a probe at the given TTL and blocks until response or timeout.
	ProbeTTL(ctx context.Context, ttl int) (mtrProbeResult, error)
	// Reset invalidates in-flight probes and clears internal caches (e.g. knownFinalTTL).
	Reset() error
	// Close releases underlying resources (sockets etc.).
	Close() error
}

// mtrSchedulerConfig configures the per-hop scheduler.
type mtrSchedulerConfig struct {
	BeginHop         int
	MaxHops          int
	HopInterval      time.Duration // delay between successive probes to the same TTL
	MaxPerHop        int           // 0 = unlimited (run until ctx cancelled)
	MaxConsecErrors  int           // per-TTL consecutive error limit; 0 → default 10
	ParallelRequests int
	ProgressThrottle time.Duration
	FillGeo          bool
	BaseConfig       Config // used for geo/RDNS lookup
	DstIP            net.IP

	IsPaused         func() bool
	IsResetRequested func() bool
}

// mtrHopState tracks per-TTL scheduling state.
type mtrHopState struct {
	completed       int
	inFlight        bool
	launchGen       uint64
	nextAt          time.Time
	disabled        bool
	consecutiveErrs int
}

// mtrCompletedProbe wraps a finished probe for the result channel.
type mtrCompletedProbe struct {
	ttl    int
	result mtrProbeResult
	gen    uint64
	doneAt time.Time
	err    error
}

// runMTRScheduler runs the per-hop independent scheduling loop.
//
// Each TTL is probed independently: after a probe completes, the next probe for
// that TTL is scheduled after HopInterval. Concurrency across TTLs is limited by
// ParallelRequests. Iteration is defined as min(Snt) over active TTLs.
//
// onSnapshot is called periodically with aggregated stats (for TUI / report).
// onProbe is called per completed probe (for raw streaming mode).
func runMTRScheduler(
	ctx context.Context,
	prober mtrTTLProber,
	agg *MTRAggregator,
	cfg mtrSchedulerConfig,
	onSnapshot MTROnSnapshot,
	onProbe func(result mtrProbeResult, iteration int),
) error {
	defer prober.Close()

	beginHop := cfg.BeginHop
	if beginHop <= 0 {
		beginHop = 1
	}
	maxHops := cfg.MaxHops
	if maxHops <= 0 {
		maxHops = 30
	}
	parallelism := cfg.ParallelRequests
	if parallelism < 1 {
		parallelism = 1
	}
	hopInterval := cfg.HopInterval
	if hopInterval <= 0 {
		hopInterval = time.Second
	}
	progressThrottle := cfg.ProgressThrottle
	if progressThrottle <= 0 {
		progressThrottle = 200 * time.Millisecond
	}

	maxConsecErrors := cfg.MaxConsecErrors
	if maxConsecErrors <= 0 {
		maxConsecErrors = 10
	}

	if beginHop > maxHops {
		return fmt.Errorf("mtr: beginHop (%d) > maxHops (%d)", beginHop, maxHops)
	}

	states := make([]mtrHopState, maxHops+1) // index by TTL [0..maxHops], 0 unused
	var generation uint64
	var knownFinalTTL int32 = -1
	var inFlight int

	resultCh := make(chan mtrCompletedProbe, parallelism*2)
	lastSnapshotTime := time.Time{}

	// effectiveMax returns the working upper bound for TTL.
	effectiveMax := func() int {
		kf := atomic.LoadInt32(&knownFinalTTL)
		if kf > 0 && int(kf) < maxHops {
			return int(kf)
		}
		return maxHops
	}

	// computeIteration returns min(Snt) over active (non-disabled) TTLs.
	computeIteration := func() int {
		eMax := effectiveMax()
		minSnt := -1
		for ttl := beginHop; ttl <= eMax; ttl++ {
			if states[ttl].disabled {
				continue
			}
			snt := states[ttl].completed
			if minSnt < 0 || snt < minSnt {
				minSnt = snt
			}
		}
		if minSnt < 0 {
			return 0
		}
		return minSnt
	}

	maybeSnapshot := func(force bool) {
		if onSnapshot == nil {
			return
		}
		now := time.Now()
		if !force && now.Sub(lastSnapshotTime) < progressThrottle {
			return
		}
		lastSnapshotTime = now
		onSnapshot(computeIteration(), agg.Snapshot())
	}

	launchProbe := func(ttl int, gen uint64) {
		states[ttl].inFlight = true
		states[ttl].launchGen = gen
		inFlight++
		go func() {
			result, err := prober.ProbeTTL(ctx, ttl)
			resultCh <- mtrCompletedProbe{
				ttl:    ttl,
				result: result,
				gen:    gen,
				doneAt: time.Now(),
				err:    err,
			}
		}()
	}

	processResult := func(cp mtrCompletedProbe) {
		inFlight--
		if cp.gen != generation {
			return // stale generation, discard silently
		}

		originTTL := cp.ttl
		if originTTL < beginHop || originTTL > maxHops {
			return
		}

		states[originTTL].inFlight = false

		// Once knownFinalTTL is determined, all probes from disabled higher TTLs
		// are discarded. Do NOT fold destination replies into finalTTL; folding
		// breaks Snt semantics and raw event fidelity.
		if states[originTTL].disabled {
			return
		}

		if cp.err != nil {
			if ctx.Err() != nil {
				return
			}
			states[originTTL].consecutiveErrs++
			fmt.Fprintf(os.Stderr, "mtr: probe error (%d/%d): %v\n",
				states[originTTL].consecutiveErrs, maxConsecErrors, cp.err)
			if states[originTTL].consecutiveErrs >= maxConsecErrors {
				// Budget exhausted: count as a completed timeout probe.
				states[originTTL].consecutiveErrs = 0
				states[originTTL].completed++
				states[originTTL].nextAt = cp.doneAt.Add(hopInterval)

				singleRes := &Result{Hops: make([][]Hop, maxHops)}
				hop := Hop{TTL: originTTL, Error: errHopLimitTimeout}
				idx := originTTL - 1
				if idx >= 0 && idx < len(singleRes.Hops) {
					singleRes.Hops[idx] = []Hop{hop}
				}
				agg.Update(singleRes, 1)

				// Emit the synthetic timeout to raw / onProbe consumers.
				if onProbe != nil {
					onProbe(mtrProbeResult{TTL: originTTL}, computeIteration())
				}

				maybeSnapshot(false)
				return
			}
			// Below budget: reschedule after interval.
			states[originTTL].nextAt = cp.doneAt.Add(hopInterval)
			return
		}

		// ── Destination detection logic ──
		// Determine if this probe reached the destination.
		if cp.result.Success && cp.result.Addr != nil {
			peerIP := mtrAddrToIP(cp.result.Addr)
			if peerIP != nil && peerIP.Equal(cfg.DstIP) {
				curFinal := atomic.LoadInt32(&knownFinalTTL)
				if curFinal < 0 {
					// First destination detection — set finalTTL
					atomic.StoreInt32(&knownFinalTTL, int32(originTTL))
					for t := originTTL + 1; t <= maxHops; t++ {
						states[t].disabled = true
					}
				} else if int32(originTTL) < curFinal {
					// Earlier TTL reached destination — lower finalTTL and
					// migrate aggregated stats from old final to new final.
					oldFinal := int(curFinal)
					atomic.StoreInt32(&knownFinalTTL, int32(originTTL))
					for t := originTTL + 1; t <= maxHops; t++ {
						states[t].disabled = true
					}
					agg.MigrateStats(oldFinal, originTTL, cfg.MaxPerHop)
					states[originTTL].completed += states[oldFinal].completed
					if cfg.MaxPerHop > 0 && states[originTTL].completed > cfg.MaxPerHop {
						states[originTTL].completed = cfg.MaxPerHop
					}
				}
				// originTTL > curFinal case: impossible here because disabled
				// TTLs are discarded above before reaching this point.
			}
		}

		// Check MaxPerHop cap.
		// In-flight probes launched before the cap was reached may return after
		// migration pushed completed to MaxPerHop — discard them.
		if cfg.MaxPerHop > 0 && states[originTTL].completed >= cfg.MaxPerHop {
			// Still update scheduling state so the origin TTL doesn't stall.
			states[originTTL].consecutiveErrs = 0
			states[originTTL].nextAt = cp.doneAt.Add(hopInterval)
			return
		}

		// Update scheduling state for origin TTL
		states[originTTL].consecutiveErrs = 0
		states[originTTL].nextAt = cp.doneAt.Add(hopInterval)
		states[originTTL].completed++

		// Feed single-probe result to aggregator
		singleRes := &Result{Hops: make([][]Hop, maxHops)}
		hop := Hop{
			Success:  cp.result.Success,
			Address:  cp.result.Addr,
			Hostname: cp.result.Hostname,
			TTL:      originTTL,
			RTT:      cp.result.RTT,
			MPLS:     cp.result.MPLS,
			Geo:      cp.result.Geo,
			Lang:     cfg.BaseConfig.Lang,
		}
		if !hop.Success && hop.Address == nil {
			hop.Error = errHopLimitTimeout
		}
		idx := originTTL - 1
		if idx >= 0 && idx < len(singleRes.Hops) {
			singleRes.Hops[idx] = []Hop{hop}
		}

		// Fill geo/RDNS only when the probe didn't already carry them
		// (ICMP engine → no geo; fallback prober → already has geo/hostname).
		if cfg.FillGeo && cp.result.Geo == nil {
			mtrFillGeoRDNS(singleRes, cfg.BaseConfig)
		}

		agg.Update(singleRes, 1)

		// Emit probe result for raw/onProbe consumers
		if onProbe != nil {
			onProbe(cp.result, computeIteration())
		}

		maybeSnapshot(false)
	}

	scheduleReady := func() {
		if cfg.IsPaused != nil && cfg.IsPaused() {
			return
		}
		now := time.Now()
		eMax := effectiveMax()
		for ttl := beginHop; ttl <= eMax; ttl++ {
			if inFlight >= parallelism {
				break
			}
			s := &states[ttl]
			if s.disabled || s.inFlight {
				continue
			}
			if cfg.MaxPerHop > 0 && s.completed >= cfg.MaxPerHop {
				continue
			}
			if !s.nextAt.IsZero() && now.Before(s.nextAt) {
				continue
			}
			launchProbe(ttl, generation)
		}
	}

	isDone := func() bool {
		if cfg.MaxPerHop <= 0 {
			return false // unlimited: runs until ctx cancelled
		}
		eMax := effectiveMax()
		for ttl := beginHop; ttl <= eMax; ttl++ {
			s := &states[ttl]
			if s.disabled {
				continue
			}
			if s.completed < cfg.MaxPerHop || s.inFlight {
				return false
			}
		}
		return true
	}

	handleReset := func() {
		if cfg.IsResetRequested == nil || !cfg.IsResetRequested() {
			return
		}
		generation++
		for i := range states {
			states[i] = mtrHopState{}
		}
		atomic.StoreInt32(&knownFinalTTL, -1)
		agg.Reset()
		_ = prober.Reset()
	}

	// Initial scheduling burst
	scheduleReady()

	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			// Drain in-flight results before returning
			deadline := time.After(5 * time.Second)
			for inFlight > 0 {
				select {
				case <-resultCh:
					inFlight--
				case <-deadline:
					goto cancelDone
				}
			}
		cancelDone:
			maybeSnapshot(true)
			return ctx.Err()

		case cp := <-resultCh:
			processResult(cp)
			if isDone() {
				maybeSnapshot(true)
				return nil
			}
			scheduleReady()

		case <-tick.C:
			handleReset()
			scheduleReady()
			if isDone() {
				maybeSnapshot(true)
				return nil
			}
		}
	}
}

// mtrAddrToIP extracts net.IP from net.Addr.
func mtrAddrToIP(addr net.Addr) net.IP {
	if addr == nil {
		return nil
	}
	switch a := addr.(type) {
	case *net.IPAddr:
		return a.IP
	case *net.UDPAddr:
		return a.IP
	case *net.TCPAddr:
		return a.IP
	}
	return nil
}
