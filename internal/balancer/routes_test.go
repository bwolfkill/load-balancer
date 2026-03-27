package balancer

import (
	"slices"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterRoutes(t *testing.T) {
	t.Parallel()

	lb := newTestLoadBalancer()
	mux := http.NewServeMux()
	RegisterRoutes(mux, lb)

	tests := []struct {
		name           string
		method         string
		path           string
		wantNotFound   bool
		acceptedStatus []int
	}{
		{
			name:           "GET /servers is registered",
			method:         http.MethodGet,
			path:           "/servers",
			acceptedStatus: []int{http.StatusOK},
		},
		{
			name:           "GET /metrics is registered",
			method:         http.MethodGet,
			path:           "/metrics",
			acceptedStatus: []int{http.StatusOK},
		},
		{
			name:           "POST /add is registered",
			method:         http.MethodGet,
			path:           "/add",
			acceptedStatus: []int{http.StatusMethodNotAllowed},
		},
		{
			name:           "POST /remove is registered",
			method:         http.MethodGet,
			path:           "/remove",
			acceptedStatus: []int{http.StatusMethodNotAllowed},
		},
		{
			name:           "GET /health is registered",
			method:         http.MethodGet,
			path:           "/health",
			acceptedStatus: []int{http.StatusBadRequest},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code == http.StatusNotFound {
				t.Errorf("route %s %s is not registered (got 404)", tc.method, tc.path)
				return
			}

			found := slices.Contains(tc.acceptedStatus, rec.Code)
			if !found {
				t.Errorf("route %s %s: status = %d, want one of %v", tc.method, tc.path, rec.Code, tc.acceptedStatus)
			}
		})
	}
}
