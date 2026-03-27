package logger

import (
	"log/slog"
	"os"
	"strings"

	"github.com/bwolfkill/load-balancer/internal/config"
)

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
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
	level = strings.ToUpper(level)
	switch level {
	case string(config.LogLevelDebug):
		return LevelDebug
	case string(config.LogLevelInfo):
		return LevelInfo
	case string(config.LogLevelWarn):
		return LevelWarn
	case string(config.LogLevelError):
		return LevelError
	default:
		return LevelWarn
	}
}
