package balancer

import "github.com/prometheus/client_golang/prometheus"

var (
	totalRequests = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "lb_requests_total",
		Help: "Total number of requests proxied by the load balancer",
	})
	successfulRequests = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "lb_requests_success_total",
		Help: "Total number of successful requests proxied by the load balancer",
	})
	failedRequests = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "lb_requests_failed_total",
		Help: "Total number of failed requests proxied by the load balancer",
	})
	activeConnections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lb_active_connections",
		Help: "Current number of active connections being proxied per backend server",
	}, []string{"server"})
	serverHealth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lb_server_health",
		Help: "Health status of backend servers (1 = healthy, 0 = unhealthy)",
	}, []string{"server"})
	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lb_request_duration_seconds",
		Help:    "HTTP request duration to backend servers in seconds",
		Buckets: append(prometheus.DefBuckets, 15, 20, 25, 30),
	}, []string{"server"})
)

func init() {
	prometheus.MustRegister(
		totalRequests,
		successfulRequests,
		failedRequests,
		activeConnections,
		serverHealth,
		requestDuration,
	)
}