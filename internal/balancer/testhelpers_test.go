package balancer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestLoadBalancer(servers ...*Server) *LoadBalancer {
	return &LoadBalancer{
		ServerPool:     newTestServerPool(servers...),
		Algorithm:      newRoundRobin(),
		Interval:       10 * time.Second,
		RequestTimeout: 5 * time.Second,
		MaxRetries:     3,
		Metrics:        newMetrics(),
	}
}

func newTestServerPool(servers ...*Server) *ServerPool {
	pool := &ServerPool{
		Servers: make(map[string]*Server, len(servers)),
		Order:   make([]*Server, len(servers)),
	}
	for i, s := range servers {
		pool.Servers[s.Address] = s
		pool.Order[i] = s
	}
	return pool
}

func newHealthyServer(addr string) *Server {
	return &Server{Address: addr, Healthy: true}
}

func newUnhealthyServer(addr string) *Server {
	return &Server{Address: addr, Healthy: false}
}

func newTestBackend(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts, newHealthyServer(ts.URL)
}

func assertStatusCode(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Errorf("status code: got %d, want %d (body: %s)", rec.Code, want, rec.Body.String())
	}
}

func assertJSONResponse(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(target); err != nil {
		t.Fatalf("failed to decode JSON response: %v (body: %s)", err, rec.Body.String())
	}
}
