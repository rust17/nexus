package test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	lb "nexus/internal/balancer"
	px "nexus/internal/proxy"
)

const (
	testResponseBody = "Hello from backend"
)

func TestProxy_ServeHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		backendHandler http.HandlerFunc
		expectStatus   int
		expectBody     string
	}{
		{
			name: "HealthyBackend",
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(testResponseBody))
			},
			expectStatus: http.StatusOK,
			expectBody:   testResponseBody,
		},
		{
			name:           "NoBackendAvailable",
			backendHandler: nil, // Do not create backend server
			expectStatus:   http.StatusServiceUnavailable,
			expectBody:     "Service unavailable",
		},
	}

	for _, tt := range tests {
		tt := tt // Prevent closure issues
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var backend *httptest.Server
			if tt.backendHandler != nil {
				backend = httptest.NewServer(tt.backendHandler)
				defer backend.Close()
			}

			balancer := lb.NewBalancer("round_robin")
			if backend != nil {
				balancer.Add(backend.URL)
			}

			proxy := px.NewProxy(balancer)

			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			proxy.ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectStatus {
				t.Errorf("Expected status code %d, got %d", tt.expectStatus, resp.StatusCode)
			}

			if !bytes.Contains(w.Body.Bytes(), []byte(tt.expectBody)) {
				t.Errorf("Expected body to contain %q, got %q", tt.expectBody, w.Body.String())
			}
		})
	}
}

func TestProxy_ErrorHandler(t *testing.T) {
	t.Parallel()

	balancer := lb.NewBalancer("round_robin")
	proxy := px.NewProxy(balancer)

	customError := false
	proxy.SetErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		customError = true
		w.WriteHeader(http.StatusBadGateway)
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	resp := w.Result()
	if !customError {
		t.Error("Custom error handler was not called")
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("Expected status code 502, got %d", resp.StatusCode)
	}
}
