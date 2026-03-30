# load-balancer

A reverse proxy load balancer written in Go. Distributes incoming HTTP traffic across a pool of backend servers using configurable algorithms, with active health checking, graceful shutdown, and a management API.

## Features

- **Load balancing algorithms**: round-robin and least-connections
- **Active health checks**: periodic background checks mark servers healthy or unhealthy
- **Automatic retries**: requests are retried with exponential backoff on failure
- **Management API**: add/remove servers and inspect pool state at runtime without restarting
- **Graceful shutdown**: drains in-flight requests before exiting on `SIGINT`/`SIGTERM`
- **Structured logging**: JSON-friendly output via `log/slog` with configurable level

## Requirements

- Go 1.22+

## Getting Started

```bash
# Clone and build
git clone https://github.com/bwolfkill/load-balancer.git
cd load-balancer
go build ./cmd/loadbalancer

# Run locally (uses .env if present, falls back to defaults)
./loadbalancer
```

For local development the load balancer defaults to port `8080` and three test backends at `localhost:8081–8083`. Start them with:

```bash
go run ./testservers/server1 &
go run ./testservers/server2 &
go run ./testservers/server3 &
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
# Run all tests with the race detector
go test -race ./...

# With coverage report
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

The race detector (`-race`) is recommended because the balancer manages concurrent access to the server pool and connection counters.

## License

MIT
