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
