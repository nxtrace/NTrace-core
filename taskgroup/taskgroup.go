package taskgroup

import (
	"sync"
)

type TaskGroup struct {
	count int
	mu    sync.Mutex
	done  []chan struct{}
}

func New() *TaskGroup {
	return &TaskGroup{
		count: 0,
		mu:    sync.Mutex{},
		done:  []chan struct{}{},
	}
}

func (t *TaskGroup) Add() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.count++
}

func (t *TaskGroup) Done() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.count-1 == 0 {
		for _, doneChannel := range t.done {
			doneChannel <- struct{}{}
		}
		t.done = []chan struct{}{}
	}
	t.count--
}

func (t *TaskGroup) Wait() {
	doneChannel := make(chan struct{})
	t.mu.Lock()
	t.done = append(t.done, doneChannel)
	t.mu.Unlock()
	<-doneChannel
}
