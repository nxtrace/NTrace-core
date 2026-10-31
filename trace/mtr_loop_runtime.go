package trace

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type mtrLoopRuntime struct {
	ctx               context.Context
	workers           *mtrWorkerSession
	prober            mtrProber
	config            Config
	opts              MTROptions
	agg               *MTRAggregator
	onSnapshot        MTROnSnapshot
	fillGeo           bool
	bo                *mtrBackoffCfg
	iteration         int
	consecutiveErrors int
	backoff           time.Duration
	pathTracker       *mtrPathTracker
}

func newMTRLoopRuntime(
	workers *mtrWorkerSession,
	prober mtrProber,
	config Config,
	opts MTROptions,
	agg *MTRAggregator,
	onSnapshot MTROnSnapshot,
	fillGeo bool,
	bo *mtrBackoffCfg,
) *mtrLoopRuntime {
	if bo == nil {
		bo = &defaultBackoff
	}
	if opts.ProgressThrottle <= 0 {
		opts.ProgressThrottle = 200 * time.Millisecond
	}
	rt := &mtrLoopRuntime{
		ctx:        workers.ctx,
		workers:    workers,
		prober:     prober,
		config:     config,
		opts:       opts,
		agg:        agg,
		onSnapshot: onSnapshot,
		fillGeo:    fillGeo,
		bo:         bo,
		backoff:    bo.Initial,
	}
	rt.pathTracker = newMTRPathTracker(opts.MaxRounds > 0, config.MaxHops, opts.OnPathEnd)
	return rt
}

func (rt *mtrLoopRuntime) run() error {
	for {
		if err := rt.snapshotContextError(); err != nil {
			return err
		}

		rt.handleReset()
		if err := rt.waitWhilePaused(); err != nil {
			return err
		}

		res, err := rt.runProbeRound()
		if err != nil {
			shouldContinue, retErr := rt.handleProbeError(err)
			if retErr != nil {
				return retErr
			}
			if shouldContinue {
				continue
			}
		}

		rt.recordSuccess(res)
		if rt.opts.MaxRounds > 0 && rt.iteration >= rt.opts.MaxRounds {
			rt.pathTracker.completeAtMaxHops()
			return nil
		}
		if err := rt.waitInterval(); err != nil {
			return err
		}
	}
}

func (rt *mtrLoopRuntime) snapshotContextError() error {
	if rt.ctx.Err() == nil {
		return nil
	}
	rt.emitSnapshot()
	return context.Cause(rt.ctx)
}

func (rt *mtrLoopRuntime) emitSnapshot() {
	if rt.onSnapshot != nil {
		rt.onSnapshot(rt.iteration, rt.snapshotStats())
	}
}

func (rt *mtrLoopRuntime) snapshotStats() []MTRHopStat {
	return rt.decorateSnapshot(rt.agg.Snapshot())
}

func (rt *mtrLoopRuntime) decorateSnapshot(stats []MTRHopStat) []MTRHopStat {
	if pathEnd := rt.pathTracker.pathEnd(); pathEnd != nil && pathEnd.Hop > 0 {
		stats = filterMTRStatsAtPathEnd(stats, pathEnd.Hop)
	}
	for i := range stats {
		stats[i].Response = mtrProbeResponseForStat(rt.pathTracker, stats[i])
	}
	return stats
}

func (rt *mtrLoopRuntime) handleReset() {
	if rt.opts.IsResetRequested == nil || !rt.opts.IsResetRequested() {
		return
	}

	if resetter, ok := rt.prober.(mtrResetter); ok {
		resetter.resetFinalTTL()
	}
	if rt.config.RefreshIPGeoSource != nil {
		rt.config.RefreshIPGeoSource()
	}
	rt.agg.Reset()
	rt.iteration = 0
	rt.consecutiveErrors = 0
	rt.backoff = rt.bo.Initial
	rt.pathTracker.reset()
}

func (rt *mtrLoopRuntime) waitWhilePaused() error {
	if rt.opts.IsPaused == nil {
		return nil
	}
	for rt.opts.IsPaused() {
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-rt.ctx.Done():
			timer.Stop()
			return rt.snapshotContextError()
		case <-timer.C:
		}
	}
	return nil
}

func (rt *mtrLoopRuntime) runProbeRound() (*Result, error) {
	peeker, canPeek := rt.prober.(mtrPeeker)
	if !canPeek || rt.onSnapshot == nil {
		return rt.prober.probeRound(rt.ctx)
	}
	return rt.runProbeRoundWithPreview(peeker)
}

func (rt *mtrLoopRuntime) runProbeRoundWithPreview(peeker mtrPeeker) (*Result, error) {
	type probeRoundResult struct {
		result *Result
		err    error
	}
	done := make(chan probeRoundResult, 1)
	rt.workers.Go("mtr.preview", func() {
		res, err := rt.prober.probeRound(rt.ctx)
		select {
		case done <- probeRoundResult{result: res, err: err}:
		case <-rt.ctx.Done():
		}
	})

	ticker := time.NewTicker(rt.opts.ProgressThrottle)
	defer ticker.Stop()

	for {
		select {
		case result := <-done:
			if rt.ctx.Err() != nil {
				return nil, rt.ctx.Err()
			}
			return result.result, result.err
		case <-ticker.C:
			rt.emitPreview(peeker)
		case <-rt.ctx.Done():
			return nil, rt.ctx.Err()
		}
	}
}

func (rt *mtrLoopRuntime) emitPreview(peeker mtrPeeker) {
	partial := peeker.peekPartialResult()
	if partial == nil {
		return
	}
	preview := rt.agg.Clone()
	rt.onSnapshot(rt.iteration+1, rt.decorateSnapshot(preview.Update(partial, 1)))
}

func (rt *mtrLoopRuntime) handleProbeError(err error) (bool, error) {
	if rt.ctx.Err() != nil {
		return false, rt.snapshotContextError()
	}
	var setup *probeSetupError
	if errors.As(err, &setup) {
		rt.emitSnapshot()
		return false, err
	}

	rt.consecutiveErrors++
	reportMTRProbeError(rt.opts.OnProbeError, rt.consecutiveErrors, rt.bo.MaxConsec, err)
	if rt.consecutiveErrors >= rt.bo.MaxConsec {
		return false, fmt.Errorf("mtr: too many consecutive errors (%d), last: %w", rt.consecutiveErrors, err)
	}

	if err := rt.waitBackoff(); err != nil {
		return false, err
	}

	rt.backoff *= 2
	if rt.backoff > rt.bo.Max {
		rt.backoff = rt.bo.Max
	}
	return true, nil
}

func (rt *mtrLoopRuntime) waitBackoff() error {
	timer := time.NewTimer(rt.backoff)
	defer timer.Stop()

	select {
	case <-rt.ctx.Done():
		return rt.snapshotContextError()
	case <-timer.C:
		return nil
	}
}

func (rt *mtrLoopRuntime) recordSuccess(res *Result) {
	if rt.fillGeo {
		mtrFillGeoRDNS(rt.workers, res, rt.config)
	}

	rt.consecutiveErrors = 0
	rt.backoff = rt.bo.Initial
	rt.iteration++
	rt.observeResultResponses(res)

	stats := rt.agg.Update(res, 1)
	if rt.onSnapshot != nil {
		rt.onSnapshot(rt.iteration, rt.decorateSnapshot(stats))
	}
}

func (rt *mtrLoopRuntime) observeResultResponses(res *Result) {
	if res == nil {
		return
	}
	for ttl := range res.responses {
		rt.pathTracker.observe(ttl, bestMTRProbeResponse(res, ttl))
	}
	rt.pathTracker.observeStopReason(res.StopReason)
}

func (rt *mtrLoopRuntime) waitInterval() error {
	if rt.opts.Interval <= 0 {
		return nil
	}

	timer := time.NewTimer(rt.opts.Interval)
	defer timer.Stop()

	select {
	case <-rt.ctx.Done():
		return rt.snapshotContextError()
	case <-timer.C:
		return nil
	}
}
