package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"log"
	"time"
)

// TODO: 1) Have your loadbalancer only serve a single server. Accept connections and forward to the server. Get that working first.

type Server struct {
	Address string
}

type ServerPool struct {
	Servers map[string]*Server
}

func (sp *ServerPool) AddServer(addr string) {
	if sp.Servers[addr] != nil {
		return
	}
	sp.Servers[addr] = &Server{Address: addr}
}

func (sp *ServerPool) RemoveServer(addr string) {
	if sp.Servers[addr] == nil {
		return
	}
	delete(sp.Servers, addr)
}

func (sp *ServerPool) GetServers() []*Server {
	servers := make([]*Server, 0, len(sp.Servers))
	for _, server := range sp.Servers {
		servers = append(servers, server)
	}
	return servers
}

func (sp *ServerPool) roundRobin() *Server {
	if len(sp.Servers) == 0 {
		return nil
	}
	return sp.GetServers()[0]
}

func (sp *ServerPool) CreateReverseProxy(w http.ResponseWriter, r *http.Request) {
	if len(sp.Servers) == 0 {
		http.Error(w, "No servers available", http.StatusServiceUnavailable)
		return
	}

	targetUrl := sp.roundRobin().Address
	url, err := url.Parse(targetUrl)
	if err != nil {
		log.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(url)
	proxy.ServeHTTP(w, r)
}

func (sp *ServerPool) AddServerHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		addr := r.FormValue("addr")
		if addr == "" {
			http.Error(w, "Address is required", http.StatusBadRequest)
			return
		}
		sp.AddServer(addr)
		fmt.Fprintf(w, "Server added: %s", addr)
	}
}

func (sp *ServerPool) RemoveServerHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		addr := r.FormValue("addr")
		if addr == "" {
			http.Error(w, "Address is required", http.StatusBadRequest)
			return
		}
		sp.RemoveServer(addr)
		fmt.Fprintf(w, "Server removed: %s", addr)
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

func (sp *ServerPool) CreateReverseProxyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sp.CreateReverseProxy(w, r)
	}
}

func main() {
	serverpool := &ServerPool{}
	serverpool.AddServer("http://localhost:8081")

	http.HandleFunc("/", serverpool.CreateReverseProxyHandler())
	http.HandleFunc("/add", serverpool.AddServerHandler())
	http.HandleFunc("/remove", serverpool.RemoveServerHandler())
	http.HandleFunc("/servers", serverpool.GetServersHandler())

	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("Error starting load balancer server:", err)
	}
}