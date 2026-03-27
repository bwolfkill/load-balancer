package balancer

import (
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
)

func TestAddConnection(t *testing.T) {
	t.Parallel()
	s := &Server{}
	for range 5 {
		AddConnection(s)
	}
	if got := atomic.LoadInt64(&s.Connections); got != 5 {
		t.Errorf("Connections = %d, want 5", got)
	}
}

func TestRemoveConnection(t *testing.T) {
	t.Parallel()
	s := &Server{}
	atomic.StoreInt64(&s.Connections, 5)
	RemoveConnection(s)
	RemoveConnection(s)
	RemoveConnection(s)
	if got := atomic.LoadInt64(&s.Connections); got != 2 {
		t.Errorf("Connections = %d, want 2", got)
	}
}

func TestSetAlive(t *testing.T) {
	t.Parallel()
	s := &Server{Healthy: false}

	setAlive(s, true)
	if !s.Healthy {
		t.Error("Healthy = false after setAlive(true), want true")
	}

	setAlive(s, false)
	if s.Healthy {
		t.Error("Healthy = true after setAlive(false), want false")
	}
}

func TestGetServers(t *testing.T) {
	t.Parallel()

	s1 := newHealthyServer("http://s1")
	s2 := newHealthyServer("http://s2")
	s3 := newHealthyServer("http://s3")
	lb := newTestLoadBalancer(s1, s2, s3)

	got := lb.GetServers()
	if len(got) != 3 {
		t.Fatalf("GetServers() len = %d, want 3", len(got))
	}

	got[0] = newHealthyServer("http://mutated")
	if lb.ServerPool.Order[0].Address != "http://s1" {
		t.Errorf("modifying GetServers() result affected the pool's Order slice")
	}
}

func TestRemoveServer(t *testing.T) {
	t.Parallel()

	t.Run("removes middle server", func(t *testing.T) {
		t.Parallel()
		s1 := newHealthyServer("http://s1")
		s2 := newHealthyServer("http://s2")
		s3 := newHealthyServer("http://s3")
		lb := newTestLoadBalancer(s1, s2, s3)

		lb.RemoveServer("http://s2")

		if len(lb.ServerPool.Order) != 2 {
			t.Errorf("Order len = %d after remove, want 2", len(lb.ServerPool.Order))
		}
		if lb.ServerPool.Servers["http://s2"] != nil {
			t.Error("server still present in Servers map after remove")
		}
		if lb.ServerPool.Order[0].Address != "http://s1" || lb.ServerPool.Order[1].Address != "http://s3" {
			t.Errorf("unexpected Order after remove: %v", lb.ServerPool.Order)
		}
	})

	t.Run("removing nonexistent server does not panic", func(t *testing.T) {
		t.Parallel()
		lb := newTestLoadBalancer(newHealthyServer("http://s1"))
		lb.RemoveServer("http://doesnotexist")
	})

	t.Run("removing from empty pool does not panic", func(t *testing.T) {
		t.Parallel()
		lb := newTestLoadBalancer()
		lb.RemoveServer("http://anything")
	})
}

// Integration test with HealthCheck and addReverseProxy
func TestAddServerIntegration(t *testing.T) {
	ts, _ := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	lb := newTestLoadBalancer()
	lb.AddServer(ts.URL)

	s := lb.ServerPool.Servers[ts.URL]
	if s == nil {
		t.Fatalf("server %s not in pool after AddServer", ts.URL)
	}
	if !s.Healthy {
		t.Error("server is not healthy after AddServer with a live backend")
	}
	if s.reverseProxy == nil {
		t.Error("reverseProxy is nil after AddServer")
	}
}

func TestServerPoolConcurrent(t *testing.T) {
	backends := make([]string, 5)
	for i := range backends {
		ts, _ := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		backends[i] = ts.URL
	}

	lb := newTestLoadBalancer()
	var wg sync.WaitGroup

	// Writers: add servers
	for _, addr := range backends {
		wg.Go(func() {
			lb.AddServer(addr)
		})
	}

	// Writers: remove servers
	for _, addr := range backends {
		wg.Go(func() {
			lb.RemoveServer(addr)
		})
	}

	// Readers: get servers
	for range 20 {
		wg.Go(func() {
			lb.GetServers()
		})
	}

	wg.Wait()
}
