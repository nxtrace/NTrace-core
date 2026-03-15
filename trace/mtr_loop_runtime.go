package trace

import (
	"context"
	"fmt"
	"os"
	"time"
)

type mtrLoopRuntime struct {
	ctx               context.Context
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
}

func newMTRLoopRuntime(
	ctx context.Context,
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
	return &mtrLoopRuntime{
		ctx:        ctx,
		prober:     prober,
		config:     config,
		opts:       opts,
		agg:        agg,
		onSnapshot: onSnapshot,
		fillGeo:    fillGeo,
		bo:         bo,
		backoff:    bo.Initial,
	}
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
	return rt.ctx.Err()
}

func (rt *mtrLoopRuntime) emitSnapshot() {
	if rt.onSnapshot != nil {
		rt.onSnapshot(rt.iteration, rt.agg.Snapshot())
	}
}

func (rt *mtrLoopRuntime) handleReset() {
	if rt.opts.IsResetRequested == nil || !rt.opts.IsResetRequested() {
		return
	}

	rt.agg.Reset()
	rt.iteration = 0
	rt.consecutiveErrors = 0
	rt.backoff = rt.bo.Initial
	if resetter, ok := rt.prober.(mtrResetter); ok {
		resetter.resetFinalTTL()
	}
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
	var (
		res *Result
		err error
	)

	done := make(chan struct{})
	go func() {
		res, err = rt.prober.probeRound(rt.ctx)
		close(done)
	}()

	ticker := time.NewTicker(rt.opts.ProgressThrottle)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return res, err
		case <-ticker.C:
			rt.emitPreview(peeker)
		case <-rt.ctx.Done():
			<-done
			if err == nil && rt.ctx.Err() != nil {
				err = rt.ctx.Err()
			}
			return res, err
		}
	}
}

func (rt *mtrLoopRuntime) emitPreview(peeker mtrPeeker) {
	partial := peeker.peekPartialResult()
	if partial == nil {
		return
	}
	preview := rt.agg.Clone()
	rt.onSnapshot(rt.iteration+1, preview.Update(partial, 1))
}

func (rt *mtrLoopRuntime) handleProbeError(err error) (bool, error) {
	if rt.ctx.Err() != nil {
		return false, rt.snapshotContextError()
	}

	rt.consecutiveErrors++
	fmt.Fprintf(os.Stderr, "mtr: probe error (%d/%d): %v\n", rt.consecutiveErrors, rt.bo.MaxConsec, err)
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
		mtrFillGeoRDNS(res, rt.config)
	}

	rt.consecutiveErrors = 0
	rt.backoff = rt.bo.Initial
	rt.iteration++

	stats := rt.agg.Update(res, 1)
	if rt.onSnapshot != nil {
		rt.onSnapshot(rt.iteration, stats)
	}
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
