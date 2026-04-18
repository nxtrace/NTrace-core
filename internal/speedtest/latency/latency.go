package latency

import (
	"context"
	"math"
	"sort"
	"sync"
)

type ProbeFunc func(context.Context) (float64, error)

type Stats struct {
	Min    float64
	Avg    float64
	Median float64
	Max    float64
	Jitter float64
	N      int
}

func MeasureIdle(ctx context.Context, n int, probe ProbeFunc) Stats {
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
	for {
		if p.ctx.Err() != nil {
			return
		}
		v, err := p.probe(p.ctx)
		if err == nil && v >= 0 {
			p.mu.Lock()
			p.samples = append(p.samples, v)
			p.mu.Unlock()
		}
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
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	var sum float64
	for _, v := range sorted {
		sum += v
	}
	avg := sum / float64(n)
	min := sorted[0]
	max := sorted[n-1]

	med := sorted[n/2]
	if n%2 == 0 {
		med = (sorted[n/2-1] + sorted[n/2]) / 2
	}

	var jitter float64
	if n > 1 {
		for i := 1; i < n; i++ {
			jitter += math.Abs(sorted[i] - sorted[i-1])
		}
		jitter /= float64(n - 1)
	}

	return Stats{
		Min:    round2(min),
		Avg:    round2(avg),
		Median: round2(med),
		Max:    round2(max),
		Jitter: round2(jitter),
		N:      n,
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
