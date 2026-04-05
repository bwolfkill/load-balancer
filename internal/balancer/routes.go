package balancer

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func RegisterRoutes(mux *http.ServeMux, lb *LoadBalancer) {
	mux.HandleFunc("/", lb.LoadBalance)
	mux.HandleFunc("/add", lb.AddServerHandler)
	mux.HandleFunc("/remove", lb.RemoveServerHandler)
	mux.HandleFunc("/servers", lb.GetServersHandler)
	mux.HandleFunc("/health", lb.GetHealthCheckHandler)
	mux.HandleFunc("/metrics", lb.Metrics.GetMetricsHandler)
	mux.Handle("/metrics/prometheus", promhttp.Handler())
}
