package proxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	lb "nexus/internal/balancer"
)

const (
	testResponseBody = "Hello from backend"
)

func TestProxy_RequestFlow(t *testing.T) {
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
			backendHandler: nil,
			expectStatus:   http.StatusServiceUnavailable,
			expectBody:     "Service unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var backend *httptest.Server
			if tt.backendHandler != nil {
				backend = httptest.NewServer(tt.backendHandler)
				defer backend.Close()
			}

			balancer := lb.NewBalancer("round_robin")
			if backend != nil {
				balancer.Add(backend.URL)
			}

			proxy := NewProxy(balancer)

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			proxy.ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectStatus {
				t.Errorf("Expected status %d, got %d", tt.expectStatus, resp.StatusCode)
			}
			if !bytes.Contains(w.Body.Bytes(), []byte(tt.expectBody)) {
				t.Errorf("Expected body %q, got %q", tt.expectBody, w.Body.String())
			}
		})
	}
}

func TestProxy_Concurrency(t *testing.T) {
	balancer := lb.NewBalancer("round_robin")
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.Write([]byte(testResponseBody))
	}))
	defer backend.Close()
	balancer.Add(backend.URL)

	proxy := NewProxy(balancer)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			proxy.ServeHTTP(w, req)
		}()
	}
	wg.Wait()
}

func TestProxy_CustomTransport(t *testing.T) {
	balancer := lb.NewBalancer("round_robin")
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testResponseBody))
	}))
	defer backend.Close()
	balancer.Add(backend.URL)

	proxy := NewProxy(balancer)

	customTransport := &http.Transport{
		ResponseHeaderTimeout: 1 * time.Second,
	}
	proxy.SetTransport(customTransport)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	if proxy.transport != customTransport {
		t.Error("Custom transport not set properly")
	}
}

func TestProxy_ErrorHandler(t *testing.T) {
	tests := []struct {
		name         string
		setupHandler func(*Proxy)
		expectStatus int
	}{
		{
			name:         "DefaultHandler",
			setupHandler: func(p *Proxy) {},
			expectStatus: http.StatusServiceUnavailable,
		},
		{
			name: "CustomHandler",
			setupHandler: func(p *Proxy) {
				p.SetErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
					w.WriteHeader(http.StatusBadGateway)
					w.Write([]byte("custom error"))
				})
			},
			expectStatus: http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balancer := lb.NewBalancer("round_robin")
			proxy := NewProxy(balancer)
			tt.setupHandler(proxy)

			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			proxy.ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectStatus {
				t.Errorf("Expected status %d, got %d", tt.expectStatus, resp.StatusCode)
			}
		})
	}
}
