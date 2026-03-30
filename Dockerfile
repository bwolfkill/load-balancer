# Multi-stage Dockerfile for building and running the Go load balancer service

# Build stage: Use the official Go image to build the application
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG CMD_PATH
RUN go build -o /bin/service ./${CMD_PATH}

# Final stage: Use a minimal base image to run the service
FROM alpine:latest
COPY --from=builder /bin/service /bin/service
CMD ["/bin/service"]