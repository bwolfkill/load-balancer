package logger

import (
	"log/slog"
	"testing"

	"github.com/bwolfkill/load-balancer/internal/config"
)

func TestParseLogLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"", slog.LevelWarn},
		{"garbage", slog.LevelWarn},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := parseLogLevel(tc.input)
			if got != tc.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestInitializeLogger(t *testing.T) {
	levels := []string{"DEBUG", "INFO", "WARN", "ERROR", ""}
	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			cfg := &config.Config{LogLevel: level}
			InitializeLogger(cfg)
		})
	}
}
