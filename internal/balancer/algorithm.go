package balancer

import (
	"sync/atomic"
)

type Algorithm interface {
	Select([]*Server) *Server
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

func (r *RoundRobin) Select(servers []*Server) *Server {
	l := len(servers)
	if l == 0 {
		return nil
	}
	start := atomic.LoadUint64(&r.current) + 1
	for i := uint64(0); i < uint64(l); i++ {
		index := (start + i) % uint64(l)
		server := servers[index]
		if server.Healthy {
			atomic.StoreUint64(&r.current, index)
			return server
		}
	}
	return nil
}

func (l *LeastConnections) Select(servers []*Server) *Server {
	if len(servers) == 0 {
		return nil
	}
	var server *Server
	minConnections := int64(1<<63 - 1)
	for _, s := range servers {
		if s.Connections < minConnections && s.Healthy {
			minConnections = s.Connections
			server = s
		}
	}
	if minConnections == int64(1<<63-1) {
		return nil
	}
	return server
}