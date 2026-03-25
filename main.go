package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	Attempt int = iota
	Retry
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

type registerServerRequest struct {
	Addr string `json:"addr"`
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
		slog.Info("Server is unhealthy", "address", s.Address)
	} else {
		slog.Info("Server is healthy", "address", s.Address)
	}
	targetUrl := s.Address
	addReverseProxy(s, targetUrl, lb)
}

func addReverseProxy(s *Server, targetUrl string, lb *LoadBalancer) {
	url, err := url.Parse(targetUrl)
	if err != nil {
		slog.Error("Error parsing server URL", "error", err, "address", s.Address)
		return
	}
	s.reverseProxy = httputil.NewSingleHostReverseProxy(url)
	s.reverseProxy.ErrorHandler = ReverseProxyErrorHandler(lb)
	s.reverseProxy.ModifyResponse = func(resp *http.Response) error {
		lb.Metrics.RecordRequest(true)
		return nil
	}
}

func ReverseProxyErrorHandler(lb *LoadBalancer) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, e error) {
		slog.Info("Reverse proxy error", "error", e, "remoteAddr", r.RemoteAddr, "path", r.URL.Path)
		retries := GetRetryFromContext(r)
		server := lb.ServerPool.Servers[r.URL.Host]
		if server == nil {
			http.Error(w, "Server not found", http.StatusBadGateway)
			return
		}
		if retries < lb.MaxRetries {
			time.Sleep(backoffDuration(retries))
			ctx := context.WithValue(r.Context(), Retry, retries+1)
			server.reverseProxy.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		setAlive(server, false)
		lb.Metrics.RecordRequest(false)

		attempts := GetAttemptFromContext(r)
		slog.Info("Attempting retry", "remoteAddr", r.RemoteAddr, "path", r.URL.Path, "attempts", attempts)
		ctx := context.WithValue(r.Context(), Attempt, attempts+1)
		lb.LoadBalance(w, r.WithContext(ctx))
	}
}

func backoffDuration(retries int) time.Duration {
	if retries == 0 {
		return 0
	}
	duration := 100.0
	backoff := time.Duration(duration*math.Pow(2, float64(retries))) * time.Millisecond
	backoff = time.Duration(math.Min(float64((duration*math.Pow(2, float64(retries)))), float64(5000))) * time.Millisecond
	return backoff
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

func (lb *LoadBalancer) GetServers() []*Server {
	lb.ServerPool.mux.RLock()
	defer lb.ServerPool.mux.RUnlock()
	out := make([]*Server, len(lb.ServerPool.Order))
	copy(out, lb.ServerPool.Order)
	return out
}

func (lb *LoadBalancer) AddServerHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req registerServerRequest
		if r.Header.Get("Content-Type") == "application/json" {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
		} else {
			req.Addr = r.FormValue("addr")
		}
		if req.Addr == "" {
			http.Error(w, "Address is required", http.StatusBadRequest)
			return
		}
		lb.AddServer(req.Addr)
		fmt.Fprintf(w, "Server added: %s", req.Addr)
	}
}

func (lb *LoadBalancer) RemoveServerHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req registerServerRequest
		if r.Header.Get("Content-Type") == "application/json" {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
		} else {
			req.Addr = r.FormValue("addr")
		}
		if req.Addr == "" {
			http.Error(w, "Address is required", http.StatusBadRequest)
			return
		}
		lb.RemoveServer(req.Addr)
		fmt.Fprintf(w, "Server removed: %s", req.Addr)
	}
}

type ServerResponse struct {
	Address string `json:"address"`
	Healthy bool   `json:"healthy"`
}

func (lb *LoadBalancer) GetServersHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		servers := lb.GetServers()
		response := make([]ServerResponse, 0)
		for _, server := range servers {
			response = append(response, ServerResponse{Address: server.Address, Healthy: server.Healthy})
		}
		jsonHandler(w, response)
	}
}

func jsonHandler(w http.ResponseWriter, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func setAlive(s *Server, alive bool) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.Healthy = alive
}

func isAlive(s *Server) bool {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(s.Address + "/healthz")
	if err != nil {
		slog.Error("Health check failed", "error", err, "address", s.Address)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("Unhealthy status code", "statusCode", resp.StatusCode, "address", s.Address)
		return false
	}

	return true
}

func HealthCheck(s *Server) bool {
	alive := isAlive(s)
	setAlive(s, alive)
	return alive
}

func (lb *LoadBalancer) RunHealthCheck() {
	for {
		time.Sleep(lb.Interval)
		for _, server := range lb.GetServers() {
			healthy := HealthCheck(server)
			status := "up"
			if !healthy {
				status = "down"
			}
			slog.Info("Health check", "server", server.Address, "status", status)
		}
	}
}

func (lb *LoadBalancer) GetHealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	server := lb.ServerPool.Servers[r.URL.Query().Get("addr")]
	if server == nil {
		http.Error(w, "Server not found", http.StatusBadRequest)
		return
	}
	healthy := HealthCheck(server)
	fmt.Fprintf(w, "Server %s is: %t", server.Address, healthy)
}

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

func GetAttemptFromContext(r *http.Request) int {
	if attempts, ok := r.Context().Value(Attempt).(int); ok {
		return attempts
	}
	return 1
}

func GetRetryFromContext(r *http.Request) int {
	if retries, ok := r.Context().Value(Retry).(int); ok {
		return retries
	}
	return 0
}

func AddConnection(s *Server) {
	atomic.AddInt64(&s.Connections, 1)
}

func RemoveConnection(s *Server) {
	atomic.AddInt64(&s.Connections, -1)
}

type LoadBalancer struct {
	ServerPool     *ServerPool
	Algorithm      Algorithm
	Interval       time.Duration
	RequestTimeout time.Duration
	MaxRetries     int
	Metrics        *Metrics
}

func newLoadBalancer(cfg *Config) *LoadBalancer {
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
	case string(AlgorithmRoundRobin):
		return newRoundRobin()
	case string(AlgorithmLeastConnections):
		return newLeastConnections()
	default:
		slog.Warn("Invalid algorithm specified, defaulting to round_robin", "provided", algorithm)
		return newRoundRobin()
	}
}

func Shutdown(server *http.Server, channel chan os.Signal) {
	sig := <-channel
	slog.Info("Shutdown signal received", "signal", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Shutdown error", "error", err)
	}
}

// Metrics
type Metrics struct {
	TotalRequests  int64
	TotalSuccesses int64
	TotalFailures  int64
}

func newMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) RecordRequest(success bool) {
	atomic.AddInt64(&m.TotalRequests, 1)
	if success {
		atomic.AddInt64(&m.TotalSuccesses, 1)
	} else {
		atomic.AddInt64(&m.TotalFailures, 1)
	}
}

func (m *Metrics) GetMetrics() map[string]int64 {
	return map[string]int64{
		"requests":  atomic.LoadInt64(&m.TotalRequests),
		"successes": atomic.LoadInt64(&m.TotalSuccesses),
		"failures":  atomic.LoadInt64(&m.TotalFailures),
	}
}

type MetricsResponse struct {
	Requests  int64 `json:"requests"`
	Successes int64 `json:"successes"`
	Failures  int64 `json:"failures"`
}

func (m *Metrics) GetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	var response MetricsResponse
	metrics, _ := json.Marshal(m.GetMetrics())
	json.Unmarshal(metrics, &response)
	jsonHandler(w, response)
}

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	InitializeLogger(cfg)

	slog.Info("Starting load balancer", "port", cfg.Port)
	lb := newLoadBalancer(cfg)

	http.HandleFunc("/", lb.LoadBalance)
	http.HandleFunc("/add", lb.AddServerHandler())
	http.HandleFunc("/remove", lb.RemoveServerHandler())
	http.HandleFunc("/servers", lb.GetServersHandler())
	http.HandleFunc("/health", lb.GetHealthCheckHandler)
	http.HandleFunc("/metrics", lb.Metrics.GetMetricsHandler)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: nil,
	}

	go lb.RunHealthCheck()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go Shutdown(server, sigChan)

	if err := server.ListenAndServe(); err != nil {
		slog.Error("Error starting load balancer server", "error", err)
	}
}
