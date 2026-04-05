package balancer

import (
	"sync/atomic"
)

type Metrics struct {
	TotalRequests  int64
	TotalSuccesses int64
	TotalFailures  int64
}

func newMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) RecordRequest(success bool) {
	atomic.AddInt64(&m.TotalRequests, 1)
	totalRequests.Inc()
	if success {
		atomic.AddInt64(&m.TotalSuccesses, 1)
		successfulRequests.Inc()
	} else {
		atomic.AddInt64(&m.TotalFailures, 1)
		failedRequests.Inc()
	}
}

func (m *Metrics) GetMetrics() (requests, successes, failures int64) {
	return atomic.LoadInt64(&m.TotalRequests),
		atomic.LoadInt64(&m.TotalSuccesses),
		atomic.LoadInt64(&m.TotalFailures)
}
