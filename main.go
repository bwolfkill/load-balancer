package main

import (
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

func (sp *ServerPool) AddServer(addr string) {
	if sp.Servers[addr] != nil {
		return
	}
	s := &Server{Address: addr, Connections: 0}
	sp.Servers[addr] = s
	sp.Order = append(sp.Order, s)
	health := healthCheck(s)
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
}

func (sp *ServerPool) RemoveServer(addr string) {
	if sp.Servers == nil {
		return
	}
	if sp.Servers[addr] == nil {
		return
	}
	delete(sp.Servers, addr)
	for i, s := range sp.Order {
		if s.Address == addr {
			sp.Order = slices.Concat(sp.Order[:i], sp.Order[i+1:])
			break
		}
	}
}

func (sp *ServerPool) GetServers() []*Server {
	out := make([]*Server, len(sp.Order))
	copy(out, sp.Order)
	return out
}

func (sp *ServerPool) AddServerHandler() http.HandlerFunc {
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
		sp.AddServer(req.Addr)
		fmt.Fprintf(w, "Server added: %s", req.Addr)
	}
}

func (sp *ServerPool) RemoveServerHandler() http.HandlerFunc {
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
		sp.RemoveServer(req.Addr)
		fmt.Fprintf(w, "Server removed: %s", req.Addr)
	}
}

func (sp *ServerPool) GetServersHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		servers := sp.GetServers()
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

func healthCheck(s *Server) bool {
	alive := isAlive(s)
	setAlive(s, alive)
	return alive
}

func (lb *LoadBalancer) runHealthCheck() {
	for {
		time.Sleep(lb.Interval)
		for _, server := range lb.ServerPool.GetServers() {
			healthy := healthCheck(server)
			status := "up"
			if !healthy {
				status = "down"
			}
			log.Printf("%s [%s]", server.Address, status)
		}
	}
}

func (sp *ServerPool) GetHealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	server := sp.Servers[r.URL.Query().Get("addr")]
	if server == nil {
		http.Error(w, "Server not found", http.StatusBadRequest)
		return
	}
	healthy := healthCheck(server)
	fmt.Fprintf(w, "Server %s is: %t", server.Address, healthy)
}

type Algorithm interface {
	Select([]*Server) *Server
}

type RoundRobin struct {
	current uint64
}

type LeastConnections struct{}

func (r *RoundRobin) Select(servers []*Server) *Server {
	if len(servers) == 0 {
		return nil
	}
	start := atomic.LoadUint64(&r.current) + 1
	for i := uint64(0); i < uint64(len(servers)); i++ {
		index := (start + i) % uint64(len(servers))
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

type LoadBalancer struct {
	ServerPool *ServerPool
	Algorithm  Algorithm
	Interval   time.Duration
}

func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(lb.ServerPool.Order) == 0 {
		http.Error(w, "No servers available", http.StatusServiceUnavailable)
		return
	}

	server := lb.Algorithm.Select(lb.ServerPool.Order)
	if server == nil {
		http.Error(w, "No servers available", http.StatusServiceUnavailable)
		return
	}

	atomic.AddInt64(&server.Connections, 1)
	defer atomic.AddInt64(&server.Connections, -1)
	server.reverseProxy.ServeHTTP(w, r)
}

func main() {
	serverpool := &ServerPool{Servers: make(map[string]*Server)}
	serverpool.AddServer("http://localhost:8081")
	serverpool.AddServer("http://localhost:8082")
	serverpool.AddServer("http://localhost:8083")

	lb := &LoadBalancer{
		ServerPool: serverpool,
		Algorithm:  &RoundRobin{},
		Interval:   5 * time.Second,
	}

	http.HandleFunc("/", lb.ServeHTTP)
	http.HandleFunc("/add", lb.ServerPool.AddServerHandler())
	http.HandleFunc("/remove", lb.ServerPool.RemoveServerHandler())
	http.HandleFunc("/servers", lb.ServerPool.GetServersHandler())
	http.HandleFunc("/health", lb.ServerPool.GetHealthCheckHandler)

	go lb.runHealthCheck()

	log.Println("Starting Load Balancer on port 8080")
	if err := http.ListenAndServe("localhost:8080", nil); err != nil {
		log.Fatal("Error starting load balancer server:", err)
	}
}
