package balancer

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestRoundRobinSelect(t *testing.T) {
	t.Parallel()

	s1 := newHealthyServer("http://s1")
	s2 := newHealthyServer("http://s2")
	s3 := newHealthyServer("http://s3")
	unhealthy := newUnhealthyServer("http://unhealthy")

	tests := []struct {
		name  string
		pool  *ServerPool
		calls int
		want  string
	}{
		{
			name:  "empty pool returns nil",
			pool:  newTestServerPool(),
			calls: 1,
			want:  "",
		},
		{
			name:  "single healthy server",
			pool:  newTestServerPool(s1),
			calls: 1,
			want:  s1.Address,
		},
		{
			name:  "single unhealthy server returns nil",
			pool:  newTestServerPool(unhealthy),
			calls: 1,
			want:  "",
		},
		{
			name:  "all unhealthy returns nil",
			pool:  newTestServerPool(unhealthy, newUnhealthyServer("http://u2"), newUnhealthyServer("http://u3")),
			calls: 1,
			want:  "",
		},
		{
			name:  "skips unhealthy, picks first healthy",
			pool:  newTestServerPool(unhealthy, s2, s3),
			calls: 1,
			want:  s2.Address,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rr := newRoundRobin()
			var got *Server
			for i := 0; i < tc.calls; i++ {
				got = rr.Select(tc.pool)
			}
			if tc.want == "" {
				if got != nil {
					t.Errorf("Select() = %v, want nil", got.Address)
				}
				return
			}
			if got == nil {
				t.Fatalf("Select() = nil, want %s", tc.want)
			}
			if got.Address != tc.want {
				t.Errorf("Select() = %s, want %s", got.Address, tc.want)
			}
		})
	}

	t.Run("wraps around on 4th call", func(t *testing.T) {
		t.Parallel()
		pool := newTestServerPool(s1, s2, s3)
		rr := newRoundRobin()
		rr.Select(pool)
		rr.Select(pool)
		rr.Select(pool)
		got := rr.Select(pool)
		if got == nil || got.Address != s1.Address {
			t.Errorf("4th Select() = %v, want %s", got, s1.Address)
		}
	})
}

func TestRoundRobinConcurrent(t *testing.T) {
	t.Parallel()

	pool := newTestServerPool(
		newHealthyServer("http://a"),
		newHealthyServer("http://b"),
		newHealthyServer("http://c"),
		newHealthyServer("http://d"),
		newHealthyServer("http://e"),
	)
	rr := newRoundRobin()

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			for range 50 {
				got := rr.Select(pool)
				if got == nil {
					t.Errorf("Select() returned nil with all-healthy pool")
					return
				}
				if !got.Healthy {
					t.Errorf("Select() returned unhealthy server %s", got.Address)
				}
			}
		})
	}
	wg.Wait()
}

func TestLeastConnectionsSelect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pool func() *ServerPool
		want string
	}{
		{
			name: "empty pool returns nil",
			pool: func() *ServerPool { return newTestServerPool() },
			want: "",
		},
		{
			name: "picks server with lowest connections",
			pool: func() *ServerPool {
				return newTestServerPool(
					&Server{Address: "http://s1", Connections: 5, Healthy: true},
					&Server{Address: "http://s2", Connections: 2, Healthy: true},
					&Server{Address: "http://s3", Connections: 8, Healthy: true},
				)
			},
			want: "http://s2",
		},
		{
			name: "skips unhealthy even if fewest connections",
			pool: func() *ServerPool {
				return newTestServerPool(
					&Server{Address: "http://s1", Connections: 1, Healthy: false},
					&Server{Address: "http://s2", Connections: 5, Healthy: true},
					&Server{Address: "http://s3", Connections: 3, Healthy: true},
				)
			},
			want: "http://s3",
		},
		{
			name: "all unhealthy returns nil",
			pool: func() *ServerPool {
				return newTestServerPool(
					&Server{Address: "http://a", Healthy: false},
					&Server{Address: "http://b", Healthy: false},
				)
			},
			want: "",
		},
		{
			name: "tie picks first encountered",
			pool: func() *ServerPool {
				return newTestServerPool(
					&Server{Address: "http://s1", Connections: 3, Healthy: true},
					&Server{Address: "http://s2", Connections: 3, Healthy: true},
					&Server{Address: "http://s3", Connections: 3, Healthy: true},
				)
			},
			want: "http://s1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lc := newLeastConnections()
			got := lc.Select(tc.pool())
			if tc.want == "" {
				if got != nil {
					t.Errorf("Select() = %v, want nil", got.Address)
				}
				return
			}
			if got == nil {
				t.Fatalf("Select() = nil, want %s", tc.want)
			}
			if got.Address != tc.want {
				t.Errorf("Select() = %s, want %s", got.Address, tc.want)
			}
		})
	}
}

func TestLeastConnectionsConcurrent(t *testing.T) {
	t.Parallel()

	pool := newTestServerPool(
		&Server{Address: "http://a", Healthy: true},
		&Server{Address: "http://b", Healthy: true},
		&Server{Address: "http://c", Healthy: true},
	)
	lc := newLeastConnections()

	var wg sync.WaitGroup

	// Writers: concurrently increment and decrement connections
	for _, s := range pool.Order {
		wg.Go(func() {
			for range 200 {
				atomic.AddInt64(&s.Connections, 1)
				atomic.AddInt64(&s.Connections, -1)
			}
		})
	}

	// Readers: concurrently call Select
	for range 50{
		wg.Go(func() {
			for range 20{
				lc.Select(pool)
			}
		})
	}

	wg.Wait()
}
