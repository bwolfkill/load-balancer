package main

import (
	"log/slog"
	"os"
)

const (
	LevelDebug = slog.LevelDebug
	LevelInfo = slog.LevelInfo
	LevelWarn = slog.LevelWarn
	LevelError = slog.LevelError
)

func InitializeLogger(cfg *Config) {
	opts := &slog.HandlerOptions{
		Level: parseLogLevel(cfg.LogLevel),
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case string(LogLevelDebug):
		return slog.LevelDebug
	case string(LogLevelInfo):
		return slog.LevelInfo
	case string(LogLevelWarn):
		return slog.LevelWarn
	case string(LogLevelError):
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}
