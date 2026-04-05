# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Development (live reload via air)
make dev          # start test backends in Docker + load balancer with air
make down         # stop Docker containers

# Build & test
make build        # build binary to ./loadbalancer
make test         # run all tests with race detector
make coverage     # run tests + open coverage report in browser
make lint         # run golangci-lint

# Run a single test
go test -race ./internal/balancer/ -run TestFunctionName

# Observability (Docker)
make observability-up    # start Prometheus + Grafana containers
make observability-down  # stop them
make load-test           # multi-phase load test against http://localhost:8080

# Kubernetes (minikube)
make k8s-setup        # start minikube, enable ingress addon, build images, apply manifests
make k8s-up           # re-apply all manifests (after manifest changes)
make k8s-rebuild      # rebuild load balancer image + rollout restart
make k8s-down         # delete all k8s resources
make k8s-tunnel       # run minikube tunnel (keep open in separate terminal)
make k8s-observability # print Prometheus + Grafana URLs
make k8s-load-test    # run load test against http://127.0.0.1:8080
```

## Architecture

The entry point is `cmd/loadbalancer/main.go`. It calls `config.LoadConfig()`, initializes the logger, creates a TCP listener, then delegates to `run()`. The `run()` function wires everything: creates a `LoadBalancer`, registers routes on an `http.ServeMux`, starts the health check loop in a goroutine, and handles graceful shutdown on `SIGINT`/`SIGTERM`.

All business logic lives in `internal/balancer/`. Key types:

- **`LoadBalancer`** (`balancer.go`) — top-level struct holding `ServerPool`, `Algorithm`, and `Metrics`. `LoadBalance()` is the main HTTP handler: it selects a server via the algorithm, tracks connections, and delegates to the server's `reverseProxy`.
- **`ServerPool`** (`server.go`) — holds servers in both a `map[string]*Server` (for O(1) lookup by address) and an `[]*Server` slice (for ordered iteration by algorithms). Protected by `sync.RWMutex`.
- **`Algorithm`** (`algorithm.go`) — interface with a single `Select(*ServerPool) *Server` method. Two implementations: `RoundRobin` (atomic counter wrapping over the slice) and `LeastConnections` (linear scan for minimum active connections among healthy servers).
- **`proxy.go`** — wraps `httputil.ReverseProxy`. The error handler implements retry logic with exponential backoff (100ms base, 2x multiplier, 5s cap). Per-server retries exhaust first (`Retry` context key), then the server is marked unhealthy and a new server is selected via a fresh `LoadBalance` call (`Attempt` context key).
- **`health.go`** — hits `<addr>/healthz` every `HealthCheckInterval`. `RunHealthCheck()` runs in a background goroutine started at server startup.
- **`prometheus.go`** — package-level vars registered via `init()`. All five metrics are global and updated directly from `server.go` and `metrics.go`.

The retry/failover flow uses two separate context keys: `Retry` tracks retries against the *same* server (with backoff), while `Attempt` tracks how many different servers have been tried. `MaxRetries` caps both individually.

## Kubernetes setup

The k8s stack uses Ingress (nginx) to expose Prometheus and Grafana at stable hostnames. The load balancer itself is `type: LoadBalancer`. All three require `make k8s-tunnel` running in a separate terminal.

One-time host setup:
```bash
echo "127.0.0.1 prometheus.local grafana.local" | sudo tee -a /etc/hosts
```

URLs once the tunnel is running:
- Load balancer: `http://127.0.0.1:8080`
- Prometheus: `http://prometheus.local`
- Grafana: `http://grafana.local`

## Configuration

Loaded from environment variables; in `ENV=local` (default), also reads `.env`. `TARGET_SERVERS` must be set in non-local environments or the process exits. In local mode, defaults to `localhost:8081-8083`. Algorithm is set at startup and cannot be changed at runtime — requires restart or k8s configmap update + rollout.
