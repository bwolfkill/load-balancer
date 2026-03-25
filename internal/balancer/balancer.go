package balancer

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/bwolfkill/load-balancer/internal/config"
)

const (
	Attempt int = iota
	Retry
)

type LoadBalancer struct {
	ServerPool     *ServerPool
	Algorithm      Algorithm
	Interval       time.Duration
	RequestTimeout time.Duration
	MaxRetries     int
	Metrics        *Metrics
}

func NewLoadBalancer(cfg *config.Config) *LoadBalancer {
	lb := &LoadBalancer{
		ServerPool:     newServerPool(),
		Algorithm:      setAlgorithm(cfg.Algorithm),
		Interval:       cfg.HealthCheckInterval,
		RequestTimeout: cfg.RequestTimeout,
		MaxRetries:     cfg.MaxRetries,
		Metrics:        newMetrics(),
	}
	for _, s := range cfg.Servers {
		lb.AddServer(s)
	}
	return lb
}

func (lb *LoadBalancer) LoadBalance(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), lb.RequestTimeout)
	defer cancel()

	r = r.WithContext(ctx)
	if len(lb.ServerPool.Order) == 0 {
		slog.Error("No servers available", "remoteAddr", r.RemoteAddr, "path", r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	attempts := GetAttemptFromContext(r)
	if attempts > lb.MaxRetries {
		slog.Error("Too many attempts", "remoteAddr", r.RemoteAddr, "path", r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}
	server := lb.Algorithm.Select(lb.ServerPool.Order)
	if server == nil {
		slog.Error("No healthy servers available", "remoteAddr", r.RemoteAddr, "path", r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	AddConnection(server)
	defer RemoveConnection(server)
	server.reverseProxy.ServeHTTP(w, r)
}

func setAlgorithm(algorithm string) Algorithm {
	switch algorithm {
	case string(config.AlgorithmRoundRobin):
		return newRoundRobin()
	case string(config.AlgorithmLeastConnections):
		return newLeastConnections()
	default:
		slog.Warn("Invalid algorithm specified, defaulting to round_robin", "provided", algorithm)
		return newRoundRobin()
	}
}
