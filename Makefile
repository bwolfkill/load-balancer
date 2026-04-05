.PHONY: dev down build test coverage setup lint load-test observability-up observability-down k8s-setup k8s-rebuild k8s-up k8s-down k8s-tunnel k8s-status k8s-observability k8s-load-test

# Install development dependencies
setup:
	go install github.com/air-verse/air@latest
	brew install golangci-lint

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

# Run linter locally
lint:
	golangci-lint run ./...

# Run a multi-phase load test: baseline traffic, concurrent slow requests, and injected failures
# Usage: make load-test [URL=http://your-lb-url]
load-test:
	@bash scripts/load-test.sh $(URL)

# Start Prometheus and Grafana alongside the running stack
observability-up:
	docker compose up -d prometheus grafana

# Stop Prometheus and Grafana
observability-down:
	docker compose stop prometheus grafana

# Start minikube, enable ingress, build all images into its daemon, and apply all manifests
k8s-setup:
	minikube start
	minikube addons enable ingress
	kubectl wait --namespace ingress-nginx \
		--for=condition=ready pod \
		--selector=app.kubernetes.io/component=controller \
		--timeout=90s
	eval $$(minikube docker-env) && \
		docker build --build-arg CMD_PATH=cmd/loadbalancer -t loadbalancer:latest . && \
		docker build --build-arg CMD_PATH=testservers/server1 -t server1:latest . && \
		docker build --build-arg CMD_PATH=testservers/server2 -t server2:latest . && \
		docker build --build-arg CMD_PATH=testservers/server3 -t server3:latest .
	kubectl apply -Rf k8s/

# Rebuild the load balancer image and roll out the update (minikube must already be running)
k8s-rebuild:
	eval $$(minikube docker-env) && \
		docker build --build-arg CMD_PATH=cmd/loadbalancer -t loadbalancer:latest .
	kubectl rollout restart deployment/loadbalancer

# Apply all Kubernetes manifests
k8s-up:
	kubectl apply -Rf k8s/

# Delete all Kubernetes resources
k8s-down:
	kubectl delete -Rf k8s/

# Run minikube tunnel to expose ingress (keep open in a separate terminal)
k8s-tunnel:
	minikube tunnel

# Show status of all pods and services
k8s-status:
	kubectl get pods,services

# Run the load test against the minikube load balancer (requires: make k8s-tunnel running)
k8s-load-test:
	@kubectl port-forward svc/server1 8081:8081 &>/dev/null & \
	 PF_PID=$$! ; \
	 sleep 1 ; \
	 bash scripts/load-test.sh http://127.0.0.1:8080 ; STATUS=$$? ; \
	 kill $$PF_PID 2>/dev/null ; \
	 exit $$STATUS

# Print Prometheus and Grafana URLs (requires: make k8s-tunnel running + /etc/hosts entries)
k8s-observability:
	@echo "Prometheus: http://prometheus.local"
	@echo "Grafana:    http://grafana.local"
