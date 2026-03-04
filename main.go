package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"sync/atomic"
	"encoding/json"
)

type Server struct {
	Address string
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
	s := &Server{Address: addr}
	sp.Servers[addr] = s
	sp.Order = append(sp.Order, s)
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
		if r.Header.Get("Content-Type") == "application/json" {  // JSON encoding
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
		} else {  // Form encoding
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

type LoadBalancer struct {
	ServerPool   *ServerPool
	RequestCount uint64
}

func (lb *LoadBalancer) roundRobin() *Server {
	servers := lb.ServerPool.GetServers()
	if len(servers) == 0 {
		return nil
	}
	idx := atomic.AddUint64(&lb.RequestCount, 1) - 1
	return servers[idx%uint64(len(servers))]
}

func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(lb.ServerPool.Order) == 0 {
		http.Error(w, "No servers available", http.StatusServiceUnavailable)
		return
	}

	targetUrl := lb.roundRobin().Address
	url, err := url.Parse(targetUrl)
	if err != nil {
		log.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(url)
	proxy.ServeHTTP(w, r)
}

func main() {
	serverpool := &ServerPool{Servers: make(map[string]*Server)}
	serverpool.AddServer("http://localhost:8081")
	serverpool.AddServer("http://localhost:8082")
	// serverpool.AddServer("http://localhost:8083")

	lb := &LoadBalancer{
		ServerPool:   serverpool,
		RequestCount: 0,
	}

	http.HandleFunc("/", lb.ServeHTTP)
	http.HandleFunc("/add", lb.ServerPool.AddServerHandler())
	http.HandleFunc("/remove", lb.ServerPool.RemoveServerHandler())
	http.HandleFunc("/servers", lb.ServerPool.GetServersHandler())

	if err := http.ListenAndServe("localhost:8080", nil); err != nil {
		fmt.Println("Error starting load balancer server:", err)
	}
}
