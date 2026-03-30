.PHONY: dev down build test coverage setup

# Install development dependencies
setup:
	go install github.com/air-verse/air@latest

# Start test backends in Docker and run the load balancer with live reload
dev:
	@which air > /dev/null 2>&1 || (echo "air is not installed. Run 'make setup' to install it." && exit 1)
	docker compose up -d server1 server2 server3
	air

# Stop and remove all Docker containers
down:
	docker compose down

# Build the load balancer binary
build:
	go build -o ./loadbalancer ./cmd/loadbalancer

# Run all tests with the race detector
test:
	go test -race ./...

# Run tests and open a coverage report in the browser
coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out
