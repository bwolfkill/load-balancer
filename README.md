# load-balancer

[![CI](https://github.com/bwolfkill/load-balancer/actions/workflows/ci.yaml/badge.svg)](https://github.com/bwolfkill/load-balancer/actions/workflows/ci.yaml)
[![Lint](https://github.com/bwolfkill/load-balancer/actions/workflows/golangci-lint.yaml/badge.svg)](https://github.com/bwolfkill/load-balancer/actions/workflows/golangci-lint.yaml)

A reverse proxy load balancer written in Go. Distributes incoming HTTP traffic across a pool of backend servers using configurable algorithms, with active health checking, graceful shutdown, and a management API.

## Features

- **Load balancing algorithms**: round-robin and least-connections
- **Active health checks**: periodic background checks mark servers healthy or unhealthy
- **Automatic retries**: requests are retried with exponential backoff on failure
- **Management API**: add/remove servers and inspect pool state at runtime without restarting
- **Graceful shutdown**: drains in-flight requests before exiting on `SIGINT`/`SIGTERM`
- **Structured logging**: JSON-friendly output via `log/slog` with configurable level
- **Prometheus metrics**: request counts, success/failure rates, active connections, and per-server health exported at `/metrics/prometheus`

## Requirements

- Go 1.25+
- Docker + Docker Compose
- [air](https://github.com/air-verse/air) (live reload, for `make dev`)
- [golangci-lint](https://golangci-lint.run/) (linting)
- [minikube](https://minikube.sigs.k8s.io/) + [kubectl](https://kubernetes.io/docs/tasks/tools/) (optional, for Kubernetes)

## Local Development

There are three ways to run the stack locally. **`make dev` is recommended** for active development.

### `make dev` — recommended

Runs the load balancer locally with live reload (`air`) while the three test backends run in Docker. Any change to `cmd/` or `internal/` recompiles and restarts the load balancer instantly — no Docker rebuild needed.

```bash
make setup   # install air and golangci-lint (one-time)
make dev     # start test backends in Docker + load balancer with live reload
```

Load balancer: `http://localhost:8080`

```bash
ctrl+c     # stop air
make down  # stop Docker containers
```

### Docker Compose

Runs the entire stack in containers. Useful for verifying the containerized build, but requires a Docker rebuild on every code change.

```bash
docker compose up --build
```

Load balancer: `http://localhost:8080`

### Kubernetes (minikube)

Runs the full stack in a local Kubernetes cluster. Best for testing k8s-specific behavior — not recommended for day-to-day development due to the slower rebuild loop. See [Running with Kubernetes](#running-with-kubernetes-minikube) below.

## Available Make Targets

| Target                    | Description                                                           |
|---------------------------|-----------------------------------------------------------------------|
| `make setup`              | Install development dependencies (air, golangci-lint)                |
| `make dev`                | Start test backends in Docker + load balancer w/ live reload          |
| `make down`               | Stop and remove all Docker containers                                 |
| `make build`              | Build the load balancer binary                                        |
| `make test`               | Run all tests with the race detector                                  |
| `make coverage`           | Run tests and open a coverage report in the browser                   |
| `make lint`               | Run linter locally                                                    |
| `make load-test`          | Run a multi-phase load test against `http://localhost:8080`           |
| `make observability-up`   | Start Prometheus + Grafana alongside the running stack                |
| `make observability-down` | Stop Prometheus and Grafana containers                                |
| `make k8s-setup`          | Start minikube, enable ingress, build all images, apply all manifests |
| `make k8s-rebuild`        | Rebuild load balancer image and roll out update                       |
| `make k8s-up`             | Apply all Kubernetes manifests                                        |
| `make k8s-down`           | Delete all Kubernetes resources                                       |
| `make k8s-tunnel`         | Run minikube tunnel — required for load balancer + ingress access     |
| `make k8s-status`         | Show status of all pods and services                                  |
| `make k8s-observability`  | Print Prometheus and Grafana URLs                                     |
| `make k8s-load-test`      | Run the multi-phase load test against the minikube cluster            |

## Observability

The load balancer exposes Prometheus metrics at `/metrics/prometheus`. Prometheus and Grafana can be started alongside either dev mode or Docker Compose:

```bash
make dev                  # or: docker compose up --build
make observability-up     # start Prometheus and Grafana
make load-test            # run a multi-phase load test
```

The Grafana datasource and dashboard are provisioned automatically — no manual setup required. Open `http://localhost:3000` and the **Load Balancer** dashboard will already be there.

The load test runs in four phases:
1. **Baseline** — 100 sequential requests to populate request rate and cumulative totals
2. **Concurrent slow requests** — 14 parallel requests to a `/slow` endpoint that holds each connection open for 25 seconds, giving Prometheus time to observe active connections
3. **Failure injection** — removes all servers from the pool, sends 10 requests (expect 503s), then restores them
4. **Unhealthy server simulation** — sets server1's `/healthz` to return 503, waits for the health check to detect it, then restores it — visible as a red tile in the Server Health panel

| Service    | URL                   |
|------------|-----------------------|
| Prometheus | http://localhost:9090 |
| Grafana    | http://localhost:3000 |

Grafana default credentials: `admin` / `admin`.

Prometheus is pre-configured to scrape the load balancer every 5 seconds. Available metrics:

| Metric                           | Type         | Description                               |
|----------------------------------|--------------|-------------------------------------------|
| `lb_requests_total`              | Counter      | Total requests proxied                    |
| `lb_requests_success_total`      | Counter      | Requests that returned a success          |
| `lb_requests_failed_total`       | Counter      | Requests that failed or were retried      |
| `lb_active_connections`          | GaugeVec     | Active connections per backend            |
| `lb_server_health`               | GaugeVec     | Health status per backend (1=up)          |
| `lb_request_duration_seconds`    | HistogramVec | Request duration per backend (p50/p95/p99)|

To stop:

```bash
make observability-down
```

## Running with Kubernetes (minikube)

### First-time setup

```bash
# Add stable hostnames for Prometheus and Grafana (one-time)
echo "127.0.0.1 prometheus.local grafana.local" | sudo tee -a /etc/hosts

# Start minikube, enable ingress, build all images, and apply all manifests
make k8s-setup
```

### Starting the tunnel

`minikube tunnel` is required to expose both the load balancer and the ingress controller to the host. Keep it running in a separate terminal:

```bash
make k8s-tunnel
```

Once the tunnel is running:
- Load balancer: `http://127.0.0.1:8080`
- Prometheus: `http://prometheus.local`
- Grafana: `http://grafana.local`

```bash
make k8s-observability   # print the observability URLs
make k8s-load-test       # run the multi-phase load test
```

### After code changes

```bash
make k8s-rebuild   # rebuilds the load balancer image and rolls out the update
```

### Updating configuration

```bash
# Edit k8s/loadbalancer/configmap.yaml, then:
kubectl apply -f k8s/loadbalancer/configmap.yaml
kubectl rollout restart deployment/loadbalancer
```

### Teardown

```bash
make k8s-down    # delete all Kubernetes resources
minikube stop    # stop the cluster
```

To suspend without losing state:

```bash
minikube pause
minikube unpause
```

## Configuration

All configuration is via environment variables. A `.env` file is loaded automatically when `ENV=local` (the default).

| Variable                 | Default          | Description                                              |
|--------------------------|------------------|----------------------------------------------------------|
| `ENV`                    | `local`          | Environment. Non-local envs require `TARGET_SERVERS`.    |
| `PORT`                   | `8080`           | Port the load balancer listens on.                       |
| `TARGET_SERVERS`         | *(see below)*    | Comma-separated list of backend URLs.                    |
| `ALGORITHM`              | `round_robin`    | `round_robin` or `least_connections`.                    |
| `HEALTH_CHECK_INTERVAL`  | `10000` (10s)    | Milliseconds, or a Go duration string (e.g. `30s`).      |
| `REQUEST_TIMEOUT`        | `30000` (30s)    | Milliseconds, or a Go duration string (e.g. `10s`).      |
| `MAX_RETRIES`            | `3`              | Maximum proxy retry attempts before returning 503.       |
| `LOG_LEVEL`              | `WARN`           | `DEBUG`, `INFO`, `WARN`, or `ERROR`. Forced to `INFO` when `ENV=local`. |

When `ENV=local` and `TARGET_SERVERS` is empty, the three localhost defaults are used. In any other environment, `TARGET_SERVERS` must be set or the process will exit.

**Example `.env`:**

```env
ENV=local
PORT=8080
TARGET_SERVERS=http://localhost:8081,http://localhost:8082,http://localhost:8083
ALGORITHM=round_robin
LOG_LEVEL=INFO
```

## Management API

All endpoints are served on the same port as the load balancer.

| Method | Path                   | Description                                       |
|--------|------------------------|---------------------------------------------------|
| `GET`  | `/servers`             | List all backend servers and their health status. |
| `GET`  | `/metrics`             | Request counts: total, successes, failures.       |
| `GET`  | `/metrics/prometheus`  | Prometheus metrics endpoint.                      |
| `GET`  | `/health`              | Check a specific server. `?addr=<url>`            |
| `POST` | `/add`                 | Add a backend server to the pool.                 |
| `POST` | `/remove`              | Remove a backend server from the pool.            |

**Add a server:**

```bash
curl -X POST http://localhost:8080/add \
  -H "Content-Type: application/json" \
  -d '{"addr": "http://localhost:8084"}'
```

**Remove a server:**

```bash
curl -X POST http://localhost:8080/remove \
  -H "Content-Type: application/json" \
  -d '{"addr": "http://localhost:8084"}'
```

**Get metrics:**

```bash
curl http://localhost:8080/metrics
# {"Requests":42,"Successes":41,"Failures":1}
```

## Project Structure

```
cmd/loadbalancer/       # main package — wires config, listener, and run()
internal/
  balancer/             # core load balancing logic, handlers, health checks
  config/               # environment-based configuration loading
  logger/               # slog initialization
testservers/            # lightweight HTTP servers for local development
observability/          # Prometheus scrape config and Grafana provisioning
k8s/                    # Kubernetes manifests for minikube
scripts/                # load test script
```

`internal/` is enforced by the Go compiler — nothing outside this module can import it. All business logic lives there to keep the `main` package thin.

## Running Tests

```bash
make test                                          # run all tests with the race detector
make coverage                                      # run tests and open a coverage report
go test -race ./internal/balancer/ -run TestName  # run a single test
make lint                                          # run linter
```

The race detector is always enabled because the balancer manages concurrent access to the server pool and connection counters. Both `test` and `lint` run automatically on every push to main via GitHub Actions.

## License

MIT
