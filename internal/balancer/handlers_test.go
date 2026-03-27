package balancer

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGetServersHandler(t *testing.T) {
	t.Parallel()

	t.Run("returns server list as JSON", func(t *testing.T) {
		t.Parallel()
		s1 := newHealthyServer("http://s1")
		s2 := newUnhealthyServer("http://s2")
		lb := newTestLoadBalancer(s1, s2)

		req := httptest.NewRequest(http.MethodGet, "/servers", nil)
		rec := httptest.NewRecorder()
		lb.GetServersHandler(rec, req)

		assertStatusCode(t, rec, http.StatusOK)

		var resp []ServerResponse
		assertJSONResponse(t, rec, &resp)
		if len(resp) != 2 {
			t.Fatalf("response len = %d, want 2", len(resp))
		}
		if resp[0].Address != "http://s1" || !resp[0].Healthy {
			t.Errorf("resp[0] = %+v, want {Address:http://s1 Healthy:true}", resp[0])
		}
		if resp[1].Address != "http://s2" || resp[1].Healthy {
			t.Errorf("resp[1] = %+v, want {Address:http://s2 Healthy:false}", resp[1])
		}
	})

	t.Run("empty pool returns empty JSON array", func(t *testing.T) {
		t.Parallel()
		lb := newTestLoadBalancer()

		req := httptest.NewRequest(http.MethodGet, "/servers", nil)
		rec := httptest.NewRecorder()
		lb.GetServersHandler(rec, req)

		assertStatusCode(t, rec, http.StatusOK)
		var resp []ServerResponse
		assertJSONResponse(t, rec, &resp)
		if len(resp) != 0 {
			t.Errorf("response len = %d, want 0", len(resp))
		}
	})

	t.Run("wrong method returns 405", func(t *testing.T) {
		t.Parallel()
		lb := newTestLoadBalancer()
		req := httptest.NewRequest(http.MethodPost, "/servers", nil)
		rec := httptest.NewRecorder()
		lb.GetServersHandler(rec, req)
		assertStatusCode(t, rec, http.StatusMethodNotAllowed)
	})
}

func TestAddServerHandler(t *testing.T) {
	t.Parallel()

	t.Run("JSON body adds server", func(t *testing.T) {
		t.Parallel()
		ts, _ := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		lb := newTestLoadBalancer()

		body, _ := json.Marshal(RegisterServerRequest{Addr: ts.URL})
		req := httptest.NewRequest(http.MethodPost, "/add", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		lb.AddServerHandler(rec, req)

		assertStatusCode(t, rec, http.StatusOK)
		if lb.ServerPool.Servers[ts.URL] == nil {
			t.Errorf("server %s was not added to pool", ts.URL)
		}
	})

	t.Run("form body adds server", func(t *testing.T) {
		t.Parallel()
		ts, _ := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		lb := newTestLoadBalancer()

		form := url.Values{"addr": {ts.URL}}
		req := httptest.NewRequest(http.MethodPost, "/add", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		lb.AddServerHandler(rec, req)

		assertStatusCode(t, rec, http.StatusOK)
	})

	t.Run("missing addr returns 400", func(t *testing.T) {
		t.Parallel()
		lb := newTestLoadBalancer()
		body := strings.NewReader("{}")
		req := httptest.NewRequest(http.MethodPost, "/add", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		lb.AddServerHandler(rec, req)
		assertStatusCode(t, rec, http.StatusBadRequest)
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		t.Parallel()
		lb := newTestLoadBalancer()
		req := httptest.NewRequest(http.MethodPost, "/add", strings.NewReader("{bad"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		lb.AddServerHandler(rec, req)
		assertStatusCode(t, rec, http.StatusBadRequest)
	})

	t.Run("wrong method returns 405", func(t *testing.T) {
		t.Parallel()
		lb := newTestLoadBalancer()
		req := httptest.NewRequest(http.MethodGet, "/add", nil)
		rec := httptest.NewRecorder()
		lb.AddServerHandler(rec, req)
		assertStatusCode(t, rec, http.StatusMethodNotAllowed)
	})
}

func TestRemoveServerHandler(t *testing.T) {
	t.Parallel()

	t.Run("removes existing server", func(t *testing.T) {
		t.Parallel()
		s := newHealthyServer("http://to-remove")
		lb := newTestLoadBalancer(s)

		body, _ := json.Marshal(RegisterServerRequest{Addr: "http://to-remove"})
		req := httptest.NewRequest(http.MethodPost, "/remove", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		lb.RemoveServerHandler(rec, req)

		assertStatusCode(t, rec, http.StatusOK)
		if lb.ServerPool.Servers["http://to-remove"] != nil {
			t.Error("server was not removed from pool")
		}
	})

	t.Run("missing addr returns 400", func(t *testing.T) {
		t.Parallel()
		lb := newTestLoadBalancer()
		req := httptest.NewRequest(http.MethodPost, "/remove", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		lb.RemoveServerHandler(rec, req)
		assertStatusCode(t, rec, http.StatusBadRequest)
	})

	t.Run("wrong method returns 405", func(t *testing.T) {
		t.Parallel()
		lb := newTestLoadBalancer()
		req := httptest.NewRequest(http.MethodGet, "/remove", nil)
		rec := httptest.NewRecorder()
		lb.RemoveServerHandler(rec, req)
		assertStatusCode(t, rec, http.StatusMethodNotAllowed)
	})
}

func TestGetMetricsHandler(t *testing.T) {
	t.Parallel()

	m := newMetrics()
	for range 3 {
		m.RecordRequest(true)
	}
	for range 2 {
		m.RecordRequest(false)
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	m.GetMetricsHandler(rec, req)

	assertStatusCode(t, rec, http.StatusOK)

	var resp MetricsResponse
	assertJSONResponse(t, rec, &resp)
	if resp.Requests != 5 || resp.Successes != 3 || resp.Failures != 2 {
		t.Errorf("MetricsResponse = %+v, want {Requests:5 Successes:3 Failures:2}", resp)
	}
}

func TestGetHealthCheckHandler(t *testing.T) {
	t.Parallel()

	t.Run("valid healthy server returns 200", func(t *testing.T) {
		t.Parallel()
		_, s := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		lb := newTestLoadBalancer(s)

		req := httptest.NewRequest(http.MethodGet, "/health?addr="+s.Address, nil)
		rec := httptest.NewRecorder()
		lb.GetHealthCheckHandler(rec, req)
		assertStatusCode(t, rec, http.StatusOK)
	})

	t.Run("unknown server address returns 400", func(t *testing.T) {
		t.Parallel()
		lb := newTestLoadBalancer()
		req := httptest.NewRequest(http.MethodGet, "/health?addr=http://unknown", nil)
		rec := httptest.NewRecorder()
		lb.GetHealthCheckHandler(rec, req)
		assertStatusCode(t, rec, http.StatusBadRequest)
	})

	t.Run("wrong HTTP method returns 405", func(t *testing.T) {
		t.Parallel()
		lb := newTestLoadBalancer()
		req := httptest.NewRequest(http.MethodPost, "/health", nil)
		rec := httptest.NewRecorder()
		lb.GetHealthCheckHandler(rec, req)
		assertStatusCode(t, rec, http.StatusMethodNotAllowed)
	})
}
