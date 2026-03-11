package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"sync"
	"sync/atomic"
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
	mux          sync.RWMutex
	reverseProxy *httputil.ReverseProxy
}

type ServerPool struct {
	Servers map[string]*Server
	Order   []*Server
}

type registerServerRequest struct {
	Addr string `json:"addr"`
}

func newServerPool() *ServerPool {
	return &ServerPool{
		Servers: make(map[string]*Server),
		Order:   []*Server{},
	}
}

func (lb *LoadBalancer) AddServer(addr string) {
	if lb.ServerPool.Servers[addr] != nil {
		return
	}
	s := &Server{Address: addr, Connections: 0}
	lb.ServerPool.Servers[addr] = s
	lb.ServerPool.Order = append(lb.ServerPool.Order, s)

	health := HealthCheck(s)
	s.Healthy = health
	if !health {
		log.Printf("Server %s is unhealthy\n", s.Address)
	} else {
		log.Printf("Server %s is healthy\n", s.Address)
	}
	targetUrl := s.Address
	url, err := url.Parse(targetUrl)
	if err != nil {
		log.Fatal(err)
	}
	s.reverseProxy = httputil.NewSingleHostReverseProxy(url)
	s.reverseProxy.ErrorHandler = ReverseProxyErrorHandler(lb)
}

func ReverseProxyErrorHandler(lb *LoadBalancer) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, e error) {
		log.Printf("[%s] %s\n", r.RemoteAddr, e.Error())
		retries := GetRetryFromContext(r)
		server := lb.ServerPool.Servers[r.URL.Host]
		if server == nil {
			http.Error(w, "Server not found", http.StatusBadGateway)
			return
		}
		if retries < 3 {
			time.Sleep(10 * time.Millisecond)
			ctx := context.WithValue(r.Context(), Retry, retries+1)
			server.reverseProxy.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		setAlive(server, false)

		attempts := GetAttemptFromContext(r)
		log.Printf("%s(%s) attempting retry %d\n", r.RemoteAddr, r.URL.Path, attempts)
		ctx := context.WithValue(r.Context(), Attempt, attempts+1)
		lb.LoadBalance(w, r.WithContext(ctx))
	}
}

func (lb *LoadBalancer) RemoveServer(addr string) {
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

func (lb *LoadBalancer) GetServersHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		servers := lb.GetServers()
		for _, server := range servers {
			fmt.Fprintf(w, "Server: %s\n", server.Address)
		}
	}
}

func setAlive(s *Server, alive bool) {
	s.mux.Lock()
	s.Healthy = alive
	s.mux.Unlock()
}

func isAlive(s *Server) bool {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(s.Address + "/healthz")
	if err != nil {
		return false
	}
	resp.Body.Close()
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
			log.Printf("%s [%s]", server.Address, status)
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

type LoadBalancer struct {
	ServerPool *ServerPool
	Algorithm  Algorithm
	Interval   time.Duration
}

func (lb *LoadBalancer) LoadBalance(w http.ResponseWriter, r *http.Request) {
	if len(lb.ServerPool.Order) == 0 {
		log.Printf("%s(%s) no servers available", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	attempts := GetAttemptFromContext(r)
	if attempts > 3 {
		log.Printf("%s(%s) too many attempts, terminating", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}
	server := lb.Algorithm.Select(lb.ServerPool.Order)
	if server == nil {
		log.Printf("%s(%s) no healthy servers available", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	atomic.AddInt64(&server.Connections, 1)
	defer atomic.AddInt64(&server.Connections, -1)
	server.reverseProxy.ServeHTTP(w, r)
}

func main() {
	lb := &LoadBalancer{
		ServerPool: newServerPool(),
		Algorithm:  newRoundRobin(),
		Interval:   5 * time.Second,
	}
	lb.AddServer("http://localhost:8081")
	lb.AddServer("http://localhost:8082")
	lb.AddServer("http://localhost:8083")

	http.HandleFunc("/", lb.LoadBalance)
	http.HandleFunc("/add", lb.AddServerHandler())
	http.HandleFunc("/remove", lb.RemoveServerHandler())
	http.HandleFunc("/servers", lb.GetServersHandler())
	http.HandleFunc("/health", lb.GetHealthCheckHandler)

	go lb.RunHealthCheck()

	log.Println("Starting Load Balancer on port 8080")
	if err := http.ListenAndServe("localhost:8080", nil); err != nil {
		log.Fatal("Error starting load balancer server:", err)
	}
}
