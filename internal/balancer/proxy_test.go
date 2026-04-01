package balancer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBackoffDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		retries int
		want    time.Duration
	}{
		{0, 0},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{5, 3200 * time.Millisecond},
		{10, 5000 * time.Millisecond},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := backoffDuration(tc.retries)
			if got != tc.want {
				t.Errorf("backoffDuration(%d) = %v, want %v", tc.retries, got, tc.want)
			}
		})
	}
}

func TestGetAttemptFromContext(t *testing.T) {
	t.Parallel()

	t.Run("no value returns default 1", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if got := GetAttemptFromContext(r); got != 1 {
			t.Errorf("GetAttemptFromContext() = %d, want 1", got)
		}
	})

	t.Run("value present returns it", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.WithContext(context.WithValue(r.Context(), Attempt, 5))
		if got := GetAttemptFromContext(r); got != 5 {
			t.Errorf("GetAttemptFromContext() = %d, want 5", got)
		}
	})
}

func TestGetRetryFromContext(t *testing.T) {
	t.Parallel()

	t.Run("no value returns default 0", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if got := GetRetryFromContext(r); got != 0 {
			t.Errorf("GetRetryFromContext() = %d, want 0", got)
		}
	})

	t.Run("value present returns it", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.WithContext(context.WithValue(r.Context(), Retry, 3))
		if got := GetRetryFromContext(r); got != 3 {
			t.Errorf("GetRetryFromContext() = %d, want 3", got)
		}
	})
}

func TestAddReverseProxy(t *testing.T) {
	t.Parallel()

	// Start a real backend that records whether it received the request
	received := make(chan struct{}, 1)
	_, s := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
		received <- struct{}{}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok from backend"))
	})

	lb := newTestLoadBalancer(s)
	addReverseProxy(s, s.Address, lb)

	if s.reverseProxy == nil {
		t.Fatal("addReverseProxy did not set s.reverseProxy")
	}

	// Fire a request through the proxy and verify the backend receives it
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.reverseProxy.ServeHTTP(rec, req)

	assertStatusCode(t, rec, http.StatusOK)

	select {
	case <-received:
	default:
		t.Error("backend did not receive the proxied request")
	}

	// ModifyResponse should have recorded 1 success
	requests, successes, failures := lb.Metrics.GetMetrics()
	if requests != 1 || successes != 1 || failures != 0 {
		t.Errorf("metrics after successful proxy: got (%d, %d, %d), want (1, 1, 0)", requests, successes, failures)
	}
}
