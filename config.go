package main

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
	"fmt"

	"github.com/joho/godotenv"
)

type Environment string
type LoadBalancerAlgorithm string
type LogLevel string

const (
	EnvironmentLocal Environment = "local"
	EnvironmentDev   Environment = "development"
	EnvironmentProd  Environment = "production"

	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"

	AlgorithmRoundRobin       LoadBalancerAlgorithm = "round_robin"
	AlgorithmLeastConnections LoadBalancerAlgorithm = "least_connections"
)

type Config struct {
	Port                string
	HealthCheckInterval time.Duration
	RequestTimeout      time.Duration
	MaxRetries          int
	Algorithm           string
	Servers             []string
	LogLevel            string
}

const (
	DefaultPort                = "8080"
	DefaultHealthCheckInterval = 10000
	DefaultRequestTimeout      = 30000
	DefaultMaxRetries          = 3
	DefaultAlgorithm           = string(AlgorithmRoundRobin)
	DefaultEnvironment         = string(EnvironmentLocal)
	DefaultLogLevel            = string(LogLevelWarn)
)

func LoadConfig() (*Config, error) {
	var servers []string
	env, exists := os.LookupEnv("ENV")
	if !exists || env == "" {
		env = DefaultEnvironment
	}
	if env == string(EnvironmentLocal) {
		err := godotenv.Load(".env")
		if err != nil {
			slog.Error("Error loading .env file, defaulting", "error", err)
		}
	}

	// Target Servers
	servers = strings.Split(getEnv("TARGET_SERVERS", ""), ",")
	if servers[0] == "" {
		servers = servers[:0]
	}
	if env == string(EnvironmentLocal) && len(servers) == 0 {
		servers = []string{"http://localhost:8081", "http://localhost:8082", "http://localhost:8083"}
	} else if env != string(EnvironmentLocal) && len(servers) == 0 {
		slog.Error("No target servers specified")
		return nil, fmt.Errorf("TARGET_SERVERS must be set in non-local environments")
	}

	// Health Check Interval
	var healthCheckInterval time.Duration
	envHCInterval := getEnv("HEALTH_CHECK_INTERVAL", strconv.Itoa(DefaultHealthCheckInterval))
	interval, err := strconv.Atoi(envHCInterval)
	if err != nil {
		slog.Info("HEALTH_CHECK_INTERVAL is not an integer, trying duration", "value", envHCInterval)
		healthCheckInterval, err = time.ParseDuration(envHCInterval)
		if err != nil {
			slog.Warn("Invalid HEALTH_CHECK_INTERVAL set, defaulting", "provided", envHCInterval, "default", DefaultHealthCheckInterval)
			healthCheckInterval = time.Duration(DefaultHealthCheckInterval) * time.Millisecond
		}
	} else {
		healthCheckInterval = time.Duration(interval) * time.Millisecond
	}

	// Request Timeout
	var requestTimeout time.Duration
	envRequestTimeout := getEnv("REQUEST_TIMEOUT", strconv.Itoa(DefaultRequestTimeout))
	timeout, err := strconv.Atoi(envRequestTimeout)
	if err != nil {
		slog.Info("REQUEST_TIMEOUT is not an integer, trying duration", "value", envRequestTimeout)
		requestTimeout, err = time.ParseDuration(envRequestTimeout)
		if err != nil {
			slog.Warn("Invalid REQUEST_TIMEOUT set, defaulting", "provided", envRequestTimeout, "default", DefaultRequestTimeout)
			requestTimeout = time.Duration(DefaultRequestTimeout) * time.Millisecond
		}
	} else {
		requestTimeout = time.Duration(timeout) * time.Millisecond
	}

	// Max Retries
	maxRetries := DefaultMaxRetries
	if v := getEnv("MAX_RETRIES", strconv.Itoa(DefaultMaxRetries)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			maxRetries = n
		}
	}

	// Load Balancer Algorithm
	algorithm := getEnv("ALGORITHM", DefaultAlgorithm)
	if algorithm != string(AlgorithmRoundRobin) && algorithm != string(AlgorithmLeastConnections) {
		slog.Warn("Invalid algorithm specified, defaulting to round_robin", "provided", algorithm)
		algorithm = DefaultAlgorithm
	}

	// Logging Level
	logLevel := getEnv("LOG_LEVEL", DefaultLogLevel)
	if env == string(EnvironmentLocal) {
		logLevel = string(LogLevelInfo)
	} else if logLevel != string(LogLevelInfo) && logLevel != string(LogLevelWarn) {
		slog.Warn("Invalid log level specified, defaulting to WARN", "provided", logLevel)
		logLevel = DefaultLogLevel
	} else {
		logLevel = DefaultLogLevel
	}

	config := &Config{
		Port:                getEnv("PORT", DefaultPort),
		HealthCheckInterval: healthCheckInterval,
		RequestTimeout:      requestTimeout,
		MaxRetries:          maxRetries,
		Algorithm:           algorithm,
		Servers:             servers,
		LogLevel:            logLevel,
	}

	return config, nil
}

func getEnv(key string, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}
