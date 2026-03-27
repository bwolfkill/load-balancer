package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwolfkill/load-balancer/internal/balancer"
	"github.com/bwolfkill/load-balancer/internal/config"
	"github.com/bwolfkill/load-balancer/internal/logger"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}
	logger.InitializeLogger(cfg)

	ln, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		slog.Error("Failed to create listener", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	slog.Info("Starting load balancer", "port", cfg.Port)

	if err := run(ctx, cfg, ln); err != nil {
		slog.Error("Load balancer exited with error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config.Config, ln net.Listener) error {
	lb := balancer.NewLoadBalancer(cfg)
	mux := http.NewServeMux()
	balancer.RegisterRoutes(mux, lb)

	server := &http.Server{
		Addr:    ln.Addr().String(),
		Handler: mux,
	}

	go lb.RunHealthCheck()

	serveErr := make(chan error, 1)
	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
		close(serveErr)
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		slog.Info("Shutdown signal received, draining connections")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-serveErr
	}
}
