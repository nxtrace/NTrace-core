package latency

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"
)

type ProbeFunc func(context.Context) (float64, error)

const (
	loadedProbeInterval = 100 * time.Millisecond
	maxLoadedSamples    = 256
)

type Stats struct {
	Min    float64
	Avg    float64
	Median float64
	Max    float64
	Jitter float64
	N      int
}

func MeasureIdle(ctx context.Context, n int, probe ProbeFunc) Stats {
	if ctx == nil {
		ctx = context.Background()
	}
	if n <= 0 || probe == nil {
		return Stats{}
	}
	samples := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		if ctx.Err() != nil {
			break
		}
		v, err := probe(ctx)
		if err == nil && v >= 0 {
			samples = append(samples, v)
		}
	}
	return Compute(samples)
}

type Probe struct {
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	probe   ProbeFunc
	samples []float64
	wg      sync.WaitGroup
}

func StartLoaded(ctx context.Context, probe ProbeFunc) *Probe {
	if ctx == nil {
		ctx = context.Background()
	}
	if probe == nil {
		return nil
	}
	ctx2, cancel := context.WithCancel(ctx)
	p := &Probe{
		ctx:    ctx2,
		cancel: cancel,
		probe:  probe,
	}
	p.wg.Add(1)
	go p.loop()
	return p
}

func (p *Probe) loop() {
	defer p.wg.Done()
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-timer.C:
		}
		if p.ctx.Err() != nil {
			return
		}
		v, err := p.probe(p.ctx)
		if err == nil && v >= 0 {
			p.mu.Lock()
			if len(p.samples) < maxLoadedSamples {
				p.samples = append(p.samples, v)
			} else {
				copy(p.samples, p.samples[1:])
				p.samples[len(p.samples)-1] = v
			}
			p.mu.Unlock()
		}
		timer.Reset(loadedProbeInterval)
	}
}

func (p *Probe) Stop() Stats {
	if p == nil {
		return Stats{}
	}
	p.cancel()
	p.wg.Wait()
	p.mu.Lock()
	samples := append([]float64(nil), p.samples...)
	p.mu.Unlock()
	return Compute(samples)
}

func Compute(samples []float64) Stats {
	n := len(samples)
	if n == 0 {
		return Stats{}
	}
	var sum float64
	for _, v := range samples {
		sum += v
	}
	avg := sum / float64(n)

	var jitter float64
	if n > 1 {
		for i := 1; i < n; i++ {
			jitter += math.Abs(samples[i] - samples[i-1])
		}
		jitter /= float64(n - 1)
	}

	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	minV := sorted[0]
	maxV := sorted[n-1]

	med := sorted[n/2]
	if n%2 == 0 {
		med = (sorted[n/2-1] + sorted[n/2]) / 2
	}

	return Stats{
		Min:    round2(minV),
		Avg:    round2(avg),
		Median: round2(med),
		Max:    round2(maxV),
		Jitter: round2(jitter),
		N:      n,
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
