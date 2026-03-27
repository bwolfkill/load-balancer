package balancer

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsAlive(t *testing.T) {
	t.Parallel()

	t.Run("healthy server returns true", func(t *testing.T) {
		t.Parallel()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()
		s := &Server{Address: ts.URL}
		if got := isAlive(s); !got {
			t.Error("isAlive() = false, want true")
		}
	})

	t.Run("unhealthy status code returns false", func(t *testing.T) {
		t.Parallel()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer ts.Close()
		s := &Server{Address: ts.URL}
		if got := isAlive(s); got {
			t.Error("isAlive() = true, want false")
		}
	})

	t.Run("unreachable server returns false", func(t *testing.T) {
		t.Parallel()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		addr := ts.URL
		ts.Close()
		s := &Server{Address: addr}
		if got := isAlive(s); got {
			t.Error("isAlive() = true for closed server, want false")
		}
	})
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()

	t.Run("healthy backend sets Healthy true", func(t *testing.T) {
		t.Parallel()
		_, s := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		s.Healthy = false
		got := HealthCheck(s)
		if !got {
			t.Error("HealthCheck() = false, want true")
		}
		if !s.Healthy {
			t.Error("s.Healthy = false after healthy check, want true")
		}
	})

	t.Run("unreachable backend sets Healthy false", func(t *testing.T) {
		t.Parallel()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		addr := ts.URL
		ts.Close()
		s := &Server{Address: addr, Healthy: true}
		got := HealthCheck(s)
		if got {
			t.Error("HealthCheck() = true for closed server, want false")
		}
		if s.Healthy {
			t.Error("s.Healthy = true after failed check, want false")
		}
	})
}
