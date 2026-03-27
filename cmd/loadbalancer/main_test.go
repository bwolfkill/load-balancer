package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/bwolfkill/load-balancer/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		Port:                "0",
		HealthCheckInterval: 10 * time.Second,
		RequestTimeout:      5 * time.Second,
		MaxRetries:          3,
		Algorithm:           "round_robin",
		Servers:             []string{},
		LogLevel:            "INFO",
	}
}

func startTestServer(t *testing.T) (baseURL string, cancel context.CancelFunc) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}

	baseURL = fmt.Sprintf("http://%s", ln.Addr().String())
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := run(ctx, testConfig(), ln); err != nil {
			t.Logf("run() returned: %v", err)
		}
	}()

	waitForServer(t, baseURL)
	return baseURL, cancel
}

func waitForServer(t *testing.T, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/servers")
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %s did not become ready within 5s", baseURL)
}

func TestRunServesRoutes(t *testing.T) {
	baseURL, cancel := startTestServer(t)
	defer cancel()

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/servers"},
		{http.MethodGet, "/metrics"},
	}

	client := &http.Client{Timeout: 3 * time.Second}
	for _, r := range routes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			req, err := http.NewRequest(r.method, baseURL+r.path, nil)
			if err != nil {
				t.Fatalf("http.NewRequest: %v", err)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			resp.Body.Close()
			if resp.StatusCode == http.StatusNotFound {
				t.Errorf("route %s %s returned 404 — handler may be nil", r.method, r.path)
			}
		})
	}
}

func TestRunShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- run(ctx, testConfig(), ln)
	}()

	waitForServer(t, fmt.Sprintf("http://%s", ln.Addr().String()))
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("run() returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("run() did not return within 5s after context cancellation")
	}
}
