package balancer

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func jsonHandler(w http.ResponseWriter, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (lb *LoadBalancer) GetServersHandler(w http.ResponseWriter, r *http.Request) {
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

func (lb *LoadBalancer) AddServerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RegisterServerRequest
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
	_, _ = fmt.Fprintf(w, "address: %s", req.Addr)
}

func (lb *LoadBalancer) RemoveServerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RegisterServerRequest
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
	_, _ = fmt.Fprintf(w, "address: %s", req.Addr)
}

func (m *Metrics) GetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	requests, successes, failures := m.GetMetrics()
	jsonHandler(w, MetricsResponse{
		Requests:  requests,
		Successes: successes,
		Failures:  failures,
	})
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
	jsonHandler(w, ServerResponse{Address: server.Address, Healthy: healthy})
}
