package balancer

import (
	"log/slog"
	"net/http/httputil"
	"slices"
	"sync"
	"sync/atomic"
)

type Server struct {
	Address      string
	Connections  int64
	Healthy      bool
	reverseProxy *httputil.ReverseProxy
	mux          sync.RWMutex
}

type ServerPool struct {
	Servers map[string]*Server
	Order   []*Server
	mux     sync.RWMutex
}

func newServerPool() *ServerPool {
	return &ServerPool{Servers: make(map[string]*Server)}
}

func (lb *LoadBalancer) AddServer(addr string) {
	lb.ServerPool.mux.Lock()
	defer lb.ServerPool.mux.Unlock()
	if lb.ServerPool.Servers[addr] != nil {
		return
	}
	s := &Server{Address: addr}
	lb.ServerPool.Servers[addr] = s
	lb.ServerPool.Order = append(lb.ServerPool.Order, s)

	health := HealthCheck(s)
	s.Healthy = health
	if !health {
		slog.Warn("Server is unhealthy", "address", s.Address)
	} else {
		slog.Info("Server is healthy", "address", s.Address)
	}
	targetUrl := s.Address
	addReverseProxy(s, targetUrl, lb)
}

func (lb *LoadBalancer) GetServers() []*Server {
	lb.ServerPool.mux.RLock()
	defer lb.ServerPool.mux.RUnlock()
	out := make([]*Server, len(lb.ServerPool.Order))
	copy(out, lb.ServerPool.Order)
	return out
}

func (lb *LoadBalancer) RemoveServer(addr string) {
	lb.ServerPool.mux.Lock()
	defer lb.ServerPool.mux.Unlock()
	if lb.ServerPool.Servers == nil {
		return
	}
	if lb.ServerPool.Servers[addr] == nil {
		return
	}
	delete(lb.ServerPool.Servers, addr)
	for i, s := range lb.ServerPool.Order {
		if s.Address == addr {
			lb.ServerPool.Order = slices.Concat(lb.ServerPool.Order[:i], lb.ServerPool.Order[i+1:])
			break
		}
	}
}

func AddConnection(s *Server) {
	atomic.AddInt64(&s.Connections, 1)
}

func RemoveConnection(s *Server) {
	atomic.AddInt64(&s.Connections, -1)
}

func setAlive(s *Server, alive bool) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.Healthy = alive
}
