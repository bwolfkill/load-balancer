package balancer

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func addReverseProxy(s *Server, targetUrl string, lb *LoadBalancer) {
	url, err := url.Parse(targetUrl)
	if err != nil {
		slog.Error("Error parsing server URL", "error", err, "address", s.Address)
		return
	}
	s.reverseProxy = httputil.NewSingleHostReverseProxy(url)
	s.reverseProxy.ErrorHandler = ReverseProxyErrorHandler(lb)
	s.reverseProxy.ModifyResponse = func(resp *http.Response) error {
		lb.Metrics.RecordRequest(true)
		return nil
	}
}

func ReverseProxyErrorHandler(lb *LoadBalancer) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, e error) {
		slog.Info("Reverse proxy error", "error", e, "remoteAddr", r.RemoteAddr, "path", r.URL.Path)
		retries := GetRetryFromContext(r)
		server := lb.ServerPool.Servers[r.URL.Host]
		if server == nil {
			http.Error(w, "Server not found", http.StatusBadGateway)
			return
		}
		if retries < lb.MaxRetries {
			time.Sleep(backoffDuration(retries))
			ctx := context.WithValue(r.Context(), Retry, retries+1)
			server.reverseProxy.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		setAlive(server, false)
		lb.Metrics.RecordRequest(false)

		attempts := GetAttemptFromContext(r)
		slog.Info("Attempting retry", "remoteAddr", r.RemoteAddr, "path", r.URL.Path, "attempts", attempts)
		ctx := context.WithValue(r.Context(), Attempt, attempts+1)
		lb.LoadBalance(w, r.WithContext(ctx))
	}
}

func backoffDuration(retries int) time.Duration {
	if retries == 0 {
		return 0
	}
	duration := 100.0
	backoff := time.Duration(duration*math.Pow(2, float64(retries))) * time.Millisecond
	backoff = time.Duration(math.Min(float64((duration*math.Pow(2, float64(retries)))), float64(5000))) * time.Millisecond
	return backoff
}

func GetAttemptFromContext(r *http.Request) int {
	if attempts, ok := r.Context().Value(Attempt).(int); ok {
		return attempts
	}
	return 1
}

func GetRetryFromContext(r *http.Request) int {
	if retries, ok := r.Context().Value(Retry).(int); ok {
		return retries
	}
	return 0
}
