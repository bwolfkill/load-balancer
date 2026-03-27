package balancer

import (
	"sync/atomic"
)


type Algorithm interface {
	Select(*ServerPool) *Server
}

type RoundRobin struct {
	current uint64
}

type LeastConnections struct{}

func newRoundRobin() *RoundRobin {
	return &RoundRobin{}
}

func newLeastConnections() *LeastConnections {
	return &LeastConnections{}
}

func (r *RoundRobin) Select(serverPool *ServerPool) *Server {
	serverPool.mux.Lock()
	defer serverPool.mux.Unlock()
	l := len(serverPool.Order)
	if l == 0 {
		return nil
	}
	start := atomic.LoadUint64(&r.current)
	for i := uint64(0); i < uint64(l); i++ {
		index := (start + i) % uint64(l)
		server := serverPool.Order[index]
		if server.Healthy {
			atomic.StoreUint64(&r.current, (index+1)%uint64(l))
			return server
		}
	}
	return nil
}

func (l *LeastConnections) Select(serverPool *ServerPool) *Server {
	serverPool.mux.Lock()
	defer serverPool.mux.Unlock()
	if len(serverPool.Order) == 0 {
		return nil
	}
	var server *Server
	minConnections := int64(1<<63 - 1)
	for _, s := range serverPool.Order {
		conns := atomic.LoadInt64(&s.Connections)
		if conns < minConnections && s.Healthy {
			minConnections = conns
			server = s
		}
	}
	if minConnections == int64(1<<63-1) {
		return nil
	}
	return server
}