package logger

import (
	"log/slog"
	"os"

	"github.com/bwolfkill/load-balancer/internal/config"
)

const (
	LevelDebug = slog.LevelDebug
	LevelInfo = slog.LevelInfo
	LevelWarn = slog.LevelWarn
	LevelError = slog.LevelError
)

func InitializeLogger(cfg *config.Config) {
	opts := &slog.HandlerOptions{
		Level: parseLogLevel(cfg.LogLevel),
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case string(config.LogLevelDebug):
		return slog.LevelDebug
	case string(config.LogLevelInfo):
		return slog.LevelInfo
	case string(config.LogLevelWarn):
		return slog.LevelWarn
	case string(config.LogLevelError):
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}
