package balancer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestSetAlgorithm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		wantType reflect.Type
	}{
		{"round_robin", reflect.TypeOf(&RoundRobin{})},
		{"least_connections", reflect.TypeOf(&LeastConnections{})},
		{"invalid", reflect.TypeOf(&RoundRobin{})},
		{"", reflect.TypeOf(&RoundRobin{})},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := setAlgorithm(tc.input)
			if reflect.TypeOf(got) != tc.wantType {
				t.Errorf("setAlgorithm(%q) type = %T, want %v", tc.input, got, tc.wantType)
			}
		})
	}
}

func TestLoadBalanceNoServers(t *testing.T) {
	t.Parallel()
	lb := newTestLoadBalancer()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	lb.LoadBalance(rec, req)
	assertStatusCode(t, rec, http.StatusServiceUnavailable)
}

func TestLoadBalanceAllUnhealthy(t *testing.T) {
	t.Parallel()
	lb := newTestLoadBalancer(
		newUnhealthyServer("http://s1"),
		newUnhealthyServer("http://s2"),
	)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	lb.LoadBalance(rec, req)
	assertStatusCode(t, rec, http.StatusServiceUnavailable)
}

func TestLoadBalanceTooManyAttempts(t *testing.T) {
	t.Parallel()
	lb := newTestLoadBalancer(newHealthyServer("http://s1"))
	lb.MaxRetries = 3

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), Attempt, lb.MaxRetries+1))
	rec := httptest.NewRecorder()
	lb.LoadBalance(rec, req)
	assertStatusCode(t, rec, http.StatusServiceUnavailable)
}

// Integration test with addReverseProxy
func TestLoadBalanceForwards(t *testing.T) {
	_, s := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello from backend"))
	})

	lb := newTestLoadBalancer(s)
	addReverseProxy(s, s.Address, lb)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	lb.LoadBalance(rec, req)

	assertStatusCode(t, rec, http.StatusOK)
	if body := rec.Body.String(); body != "hello from backend" {
		t.Errorf("body = %q, want %q", body, "hello from backend")
	}

	// ModifyResponse on the proxy records 1 success
	requests, successes, failures := lb.Metrics.GetMetrics()
	if requests != 1 || successes != 1 || failures != 0 {
		t.Errorf("metrics = (%d, %d, %d), want (1, 1, 0)", requests, successes, failures)
	}
}
