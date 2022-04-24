package parallel_limiter

import (
	"sync"
)

type ParallelLimiter struct {
	maxCount int

	mu             sync.Mutex
	currentRunning int

	waiting []chan struct{}
}

func New(count int) *ParallelLimiter {
	return &ParallelLimiter{
		maxCount: count,

		currentRunning: 0,
		waiting:        []chan struct{}{},
	}
}

func (p *ParallelLimiter) Start() chan struct{} {
	p.mu.Lock()
	if p.currentRunning+1 > p.maxCount {
		waitChan := make(chan struct{})
		p.waiting = append(p.waiting, waitChan)
		p.mu.Unlock()
		return waitChan
	}
	p.currentRunning++
	p.mu.Unlock()
	instantResolveChan := make(chan struct{})
	go func() {
		instantResolveChan <- struct{}{}
	}()
	return instantResolveChan
}

func (p *ParallelLimiter) Finished() {
	p.mu.Lock()
	if len(p.waiting) > 0 {
		first := p.waiting[0]
		p.waiting = p.waiting[1:]
		first <- struct{}{}
		p.currentRunning++
	}
	p.currentRunning--
	p.mu.Unlock()
}
