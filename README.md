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

## Requirements

- Go 1.25+
- Docker + Docker Compose
- [air](https://github.com/air-verse/air) (live reload)
- [golangci-lint](https://golangci-lint.run/) (linting)
- [minikube](https://minikube.sigs.k8s.io/) + [kubectl](https://kubernetes.io/docs/tasks/tools/) (optional, for Kubernetes)

## Getting Started

```bash
git clone https://github.com/bwolfkill/load-balancer.git
cd load-balancer

# Install air for live reload
make setup

# Start test backends in Docker + load balancer with live reload
make dev
```

`make dev` starts the three test backends in Docker and runs the load balancer locally with `air`. Any changes to `cmd/` or `internal/` will automatically recompile and restart the load balancer.

When finished:

```bash
ctrl+c     # stop air
make down  # stop Docker containers
```

## Available Make Targets

| Target            | Description                                          |
|-------------------|------------------------------------------------------|
| `make setup`      | Install development dependencies (air)               |
| `make dev`        | Start test backends in Docker + load balancer w/ air |
| `make down`       | Stop and remove all Docker containers                |
| `make build`      | Build the load balancer binary                       |
| `make test`       | Run all tests with the race detector                 |
| `make coverage`   | Run tests and open a coverage report in the browser  |
| `make lint`       | Run linter locally                                   |
| `make k8s-up`     | Apply all Kubernetes manifests                       |
| `make k8s-down`   | Delete all Kubernetes resources                      |
| `make k8s-status` | Show status of all pods and services                 |

## Running with Docker Compose

To run the full stack in Docker without `air`:

```bash
docker compose up --build
```

This builds and starts all four services. The load balancer is available at `http://localhost:8080`.

## Running with Kubernetes (minikube)

Requires minikube and kubectl installed.

```bash
# Start minikube
minikube start

# Point your Docker CLI at minikube's internal daemon
eval $(minikube docker-env)

# Build images into minikube's daemon
docker build --build-arg CMD_PATH=cmd/loadbalancer -t loadbalancer:latest .
docker build --build-arg CMD_PATH=testservers/server1 -t server1:latest .
docker build --build-arg CMD_PATH=testservers/server2 -t server2:latest .
docker build --build-arg CMD_PATH=testservers/server3 -t server3:latest .

# Apply all manifests
kubectl apply -f k8s/

# Expose the load balancer and get the URL
minikube service loadbalancer --url
```

To update configuration (e.g. change the algorithm):

```bash
# Edit k8s/configmap.yaml, then apply and restart
kubectl apply -f k8s/configmap.yaml
kubectl rollout restart deployment/loadbalancer
```

To pause minikube:

```bash
# Suspend the cluster without losing state
  minikube pause

# Resume
minikube unpause
```

To tear down:

```bash
kubectl delete -f k8s/
minikube stop
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

| Method | Path       | Description                                      |
|--------|------------|--------------------------------------------------|
| `GET`  | `/servers` | List all backend servers and their health status.|
| `GET`  | `/metrics` | Request counts: total, successes, failures.      |
| `GET`  | `/health`  | Check a specific server. `?addr=<url>`           |
| `POST` | `/add`     | Add a backend server to the pool.                |
| `POST` | `/remove`  | Remove a backend server from the pool.           |

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
```

`internal/` is enforced by the Go compiler — nothing outside this module can import it. All business logic lives there to keep the `main` package thin.

## Running Tests

```bash
make test      # run all tests with the race detector
make coverage  # run tests and open a coverage report in the browser
make lint      # run linter locally
```

The race detector is always enabled because the balancer manages concurrent access to the server pool and connection counters. Both `test` and `lint` run automatically on every push to main via GitHub Actions.

## License

MIT
