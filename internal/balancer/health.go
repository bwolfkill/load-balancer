package balancer

import (
	"log/slog"
	"net/http"
	"time"
)

func isAlive(s *Server) bool {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(s.Address + "/healthz")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
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
			status := "up"
			healthy := HealthCheck(server)
			if !healthy {
				status = "down"
				slog.Warn("Health check failed", "server", server.Address)
			} else {
				slog.Info("Health check", "server", server.Address, "status", status)
			}
		}
	}
}
