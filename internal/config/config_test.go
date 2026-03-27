package config

import (
	"testing"
	"time"
)

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		setValue string
		setEnv   bool
		fallback string
		want     string
	}{
		{
			name:     "env set returns value",
			key:      "TEST_GET_ENV_SET",
			setValue: "bar",
			setEnv:   true,
			fallback: "default",
			want:     "bar",
		},
		{
			name:     "env missing returns fallback",
			key:      "TEST_GET_ENV_MISSING",
			setEnv:   false,
			fallback: "default",
			want:     "default",
		},
		{
			name:     "env empty returns fallback",
			key:      "TEST_GET_ENV_EMPTY",
			setValue: "",
			setEnv:   true,
			fallback: "default",
			want:     "default",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setEnv {
				t.Setenv(tc.key, tc.setValue)
			}
			got := getEnv(tc.key, tc.fallback)
			if got != tc.want {
				t.Errorf("getEnv(%q, %q) = %q, want %q", tc.key, tc.fallback, got, tc.want)
			}
		})
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("ENV", "local")
	t.Setenv("TARGET_SERVERS", "")
	t.Setenv("PORT", "")
	t.Setenv("HEALTH_CHECK_INTERVAL", "")
	t.Setenv("REQUEST_TIMEOUT", "")
	t.Setenv("MAX_RETRIES", "")
	t.Setenv("ALGORITHM", "")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() returned unexpected error: %v", err)
	}

	if cfg.Port != DefaultPort {
		t.Errorf("Port = %q, want %q", cfg.Port, DefaultPort)
	}
	if cfg.MaxRetries != DefaultMaxRetries {
		t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, DefaultMaxRetries)
	}
	if cfg.Algorithm != DefaultAlgorithm {
		t.Errorf("Algorithm = %q, want %q", cfg.Algorithm, DefaultAlgorithm)
	}
	if cfg.HealthCheckInterval != time.Duration(DefaultHealthCheckInterval)*time.Millisecond {
		t.Errorf("HealthCheckInterval = %v, want %v", cfg.HealthCheckInterval, time.Duration(DefaultHealthCheckInterval)*time.Millisecond)
	}
	if cfg.RequestTimeout != time.Duration(DefaultRequestTimeout)*time.Millisecond {
		t.Errorf("RequestTimeout = %v, want %v", cfg.RequestTimeout, time.Duration(DefaultRequestTimeout)*time.Millisecond)
	}
	if len(cfg.Servers) != 3 {
		t.Errorf("Servers len = %d, want 3", len(cfg.Servers))
	}
	if cfg.LogLevel != string(LogLevelInfo) {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, LogLevelInfo)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	tests := []struct {
		name   string
		envs   map[string]string
		assert func(t *testing.T, cfg *Config)
	}{
		{
			name: "custom port",
			envs: map[string]string{"PORT": "9090", "ENV": "local"},
			assert: func(t *testing.T, cfg *Config) {
				if cfg.Port != "9090" {
					t.Errorf("Port = %q, want %q", cfg.Port, "9090")
				}
			},
		},
		{
			name: "custom servers",
			envs: map[string]string{
				"ENV":            "local",
				"TARGET_SERVERS": "http://a,http://b",
			},
			assert: func(t *testing.T, cfg *Config) {
				if len(cfg.Servers) != 2 {
					t.Fatalf("Servers len = %d, want 2", len(cfg.Servers))
				}
				if cfg.Servers[0] != "http://a" || cfg.Servers[1] != "http://b" {
					t.Errorf("Servers = %v, want [http://a http://b]", cfg.Servers)
				}
			},
		},
		{
			name: "least_connections algorithm",
			envs: map[string]string{"ENV": "local", "ALGORITHM": "least_connections"},
			assert: func(t *testing.T, cfg *Config) {
				if cfg.Algorithm != string(AlgorithmLeastConnections) {
					t.Errorf("Algorithm = %q, want %q", cfg.Algorithm, AlgorithmLeastConnections)
				}
			},
		},
		{
			name: "invalid algorithm defaults to round_robin",
			envs: map[string]string{"ENV": "local", "ALGORITHM": "banana"},
			assert: func(t *testing.T, cfg *Config) {
				if cfg.Algorithm != DefaultAlgorithm {
					t.Errorf("Algorithm = %q, want %q", cfg.Algorithm, DefaultAlgorithm)
				}
			},
		},
		{
			name: "custom max retries",
			envs: map[string]string{"ENV": "local", "MAX_RETRIES": "7"},
			assert: func(t *testing.T, cfg *Config) {
				if cfg.MaxRetries != 7 {
					t.Errorf("MaxRetries = %d, want 7", cfg.MaxRetries)
				}
			},
		},
		{
			name: "invalid max retries keeps default",
			envs: map[string]string{"ENV": "local", "MAX_RETRIES": "notanumber"},
			assert: func(t *testing.T, cfg *Config) {
				if cfg.MaxRetries != DefaultMaxRetries {
					t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, DefaultMaxRetries)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envs {
				t.Setenv(k, v)
			}
			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() unexpected error: %v", err)
			}
			tc.assert(t, cfg)
		})
	}
}

func TestLoadConfigDurationParsing(t *testing.T) {
	t.Setenv("ENV", "local")

	tests := []struct {
		name     string
		envKey   string
		value    string
		want     time.Duration
		default_ time.Duration
	}{
		{
			name:   "HEALTH_CHECK_INTERVAL integer ms",
			envKey: "HEALTH_CHECK_INTERVAL",
			value:  "5000",
			want:   5 * time.Second,
		},
		{
			name:   "HEALTH_CHECK_INTERVAL duration string",
			envKey: "HEALTH_CHECK_INTERVAL",
			value:  "2s",
			want:   2 * time.Second,
		},
		{
			name:   "HEALTH_CHECK_INTERVAL invalid falls back to default",
			envKey: "HEALTH_CHECK_INTERVAL",
			value:  "not_a_duration",
			want:   time.Duration(DefaultHealthCheckInterval) * time.Millisecond,
		},
		{
			name:   "REQUEST_TIMEOUT integer ms",
			envKey: "REQUEST_TIMEOUT",
			value:  "10000",
			want:   10 * time.Second,
		},
		{
			name:   "REQUEST_TIMEOUT duration string",
			envKey: "REQUEST_TIMEOUT",
			value:  "15s",
			want:   15 * time.Second,
		},
		{
			name:   "REQUEST_TIMEOUT invalid falls back to default",
			envKey: "REQUEST_TIMEOUT",
			value:  "bad",
			want:   time.Duration(DefaultRequestTimeout) * time.Millisecond,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.envKey, tc.value)
			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() unexpected error: %v", err)
			}
			var got time.Duration
			if tc.envKey == "HEALTH_CHECK_INTERVAL" {
				got = cfg.HealthCheckInterval
			} else {
				got = cfg.RequestTimeout
			}
			if got != tc.want {
				t.Errorf("%s=%q: duration = %v, want %v", tc.envKey, tc.value, got, tc.want)
			}
		})
	}
}

func TestLoadConfigNonLocalNoServers(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("TARGET_SERVERS", "")

	_, err := LoadConfig()
	if err == nil {
		t.Error("LoadConfig() expected error for non-local env with no servers, got nil")
	}
}

func TestLoadConfigNonLocalWithServers(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("TARGET_SERVERS", "http://backend:8080")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}
	if len(cfg.Servers) != 1 || cfg.Servers[0] != "http://backend:8080" {
		t.Errorf("Servers = %v, want [http://backend:8080]", cfg.Servers)
	}
}
