package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwolfkill/load-balancer/internal/config"
	"github.com/bwolfkill/load-balancer/internal/logger"
	"github.com/bwolfkill/load-balancer/internal/balancer"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	logger.InitializeLogger(cfg)

	slog.Info("Starting load balancer", "port", cfg.Port)
	lb := balancer.NewLoadBalancer(cfg)

	mux := http.NewServeMux()
	balancer.RegisterRoutes(mux, lb)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: nil,
	}

	go lb.RunHealthCheck()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go Shutdown(server, sigChan)

	if err := server.ListenAndServe(); err != nil {
		slog.Error("Error starting load balancer server", "error", err)
	}
}

func Shutdown(server *http.Server, channel chan os.Signal) {
	sig := <-channel
	slog.Info("Shutdown signal received", "signal", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Shutdown error", "error", err)
	}
}
