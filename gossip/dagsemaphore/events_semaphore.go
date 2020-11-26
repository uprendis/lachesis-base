package dagsemaphore

import (
	"sync"
	"time"

	"github.com/Fantom-foundation/lachesis-base/inter/dag"
)

type EventsSemaphore struct {
	processing    dag.Metric
	maxProcessing dag.Metric

	terminated bool

	mu   sync.Mutex
	cond *sync.Cond

	warning func(received dag.Metric, processing dag.Metric, releasing dag.Metric)
}

func New(maxProcessing dag.Metric, warning func(received dag.Metric, processing dag.Metric, releasing dag.Metric)) *EventsSemaphore {
	s := &EventsSemaphore{
		maxProcessing: maxProcessing,
		warning:       warning,
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

func (s *EventsSemaphore) Acquire(events dag.Metric, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	s.mu.Lock()
	defer s.mu.Unlock()
	for !s.tryAcquire(events) {
		if s.terminated || time.Now().After(deadline) {
			return false
		}
		s.cond.Wait()
	}
	return true
}

func (s *EventsSemaphore) TryAcquire(events dag.Metric) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tryAcquire(events)
}

func (s *EventsSemaphore) tryAcquire(metric dag.Metric) bool {
	tmp := s.processing
	tmp.Num += metric.Num
	tmp.Size += metric.Size
	if tmp.Num > s.maxProcessing.Num || tmp.Size > s.maxProcessing.Size {
		return false
	}
	s.processing = tmp
	return true
}

func (s *EventsSemaphore) Release(events dag.Metric) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.processing.Num < events.Num || s.processing.Size < events.Size {
		if s.warning != nil {
			s.warning(s.processing, s.processing, events)
		}
		s.processing = dag.Metric{}
	} else {
		s.processing.Num -= events.Num
		s.processing.Size -= events.Size
	}
	s.cond.Broadcast()
}

func (s *EventsSemaphore) Terminate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxProcessing = dag.Metric{}
	s.terminated = true
	s.cond.Broadcast()
}

func (s *EventsSemaphore) Processing() dag.Metric {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.processing
}

func (s *EventsSemaphore) Available() dag.Metric {
	s.mu.Lock()
	defer s.mu.Unlock()
	return dag.Metric{
		Num:  s.maxProcessing.Num - s.processing.Num,
		Size: s.maxProcessing.Size - s.processing.Size,
	}
}
