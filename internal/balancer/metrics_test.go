package balancer

import (
	"sync"
	"testing"
)

func TestRecordRequestSuccess(t *testing.T) {
	t.Parallel()
	m := newMetrics()
	for range 5 {
		m.RecordRequest(true)
	}
	requests, successes, failures := m.GetMetrics()
	if requests != 5 || successes != 5 || failures != 0 {
		t.Errorf("GetMetrics() = (%d, %d, %d), want (5, 5, 0)", requests, successes, failures)
	}
}

func TestRecordRequestFailure(t *testing.T) {
	t.Parallel()
	m := newMetrics()
	for range 3 {
		m.RecordRequest(false)
	}
	requests, successes, failures := m.GetMetrics()
	if requests != 3 || successes != 0 || failures != 3 {
		t.Errorf("GetMetrics() = (%d, %d, %d), want (3, 0, 3)", requests, successes, failures)
	}
}

func TestRecordRequestMixed(t *testing.T) {
	t.Parallel()
	m := newMetrics()
	m.RecordRequest(true)
	m.RecordRequest(true)
	m.RecordRequest(false)
	requests, successes, failures := m.GetMetrics()
	if requests != 3 || successes != 2 || failures != 1 {
		t.Errorf("GetMetrics() = (%d, %d, %d), want (3, 2, 1)", requests, successes, failures)
	}
}

func TestMetricsConcurrent(t *testing.T) {
	t.Parallel()

	const numSuccesses = 500
	const numFailures = 300

	m := newMetrics()
	var wg sync.WaitGroup

	for range numSuccesses {
		wg.Go(func() {
			m.RecordRequest(true)
		})
	}

	for range numFailures {
		wg.Go(func() {
			m.RecordRequest(false)
		})
	}

	wg.Wait()

	requests, successes, failures := m.GetMetrics()
	total := numSuccesses + numFailures
	if requests != int64(total) {
		t.Errorf("TotalRequests = %d, want %d", requests, total)
	}
	if successes != int64(numSuccesses) {
		t.Errorf("TotalSuccesses = %d, want %d", successes, numSuccesses)
	}
	if failures != int64(numFailures) {
		t.Errorf("TotalFailures = %d, want %d", failures, numFailures)
	}
}
