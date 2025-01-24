package test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nexus/internal"
)

func TestIntegration(t *testing.T) {
	// Create test backend servers
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Response from backend 1"))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Response from backend 2"))
	}))
	defer backend2.Close()

	// Create configuration
	cfg := internal.NewConfig()
	cfg.Servers = []string{backend1.URL, backend2.URL}
	cfg.HealthCheck.Interval = 100 * time.Millisecond
	cfg.HealthCheck.Timeout = 1 * time.Second

	// Initialize load balancer
	balancer := internal.NewRoundRobinBalancer()
	for _, server := range cfg.GetServers() {
		balancer.Add(server)
	}
	balancer.Next() // Ensure starting from the first server

	// Initialize health checker
	healthChecker := internal.NewHealthChecker(cfg.GetHealthCheckConfig().Interval, cfg.GetHealthCheckConfig().Timeout)
	for _, server := range cfg.GetServers() {
		healthChecker.AddServer(server)
	}
	go healthChecker.Start()
	defer healthChecker.Stop()

	// Initialize reverse proxy
	proxy := internal.NewProxy(balancer)

	t.Run("Single request should return success status code", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		proxy.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
		}
	})

	t.Run("Load balancer should correctly round-robin backend servers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)

		// Define test cases
		testCases := []struct {
			name     string
			expected string
		}{
			{"First Request", "Response from backend 1"},
			{"Second Request", "Response from backend 2"},
			{"Third Request", "Response from backend 1"},
			{"Fourth Request", "Response from backend 2"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				proxy.ServeHTTP(w, req)

				got := w.Body.String()
				if got != tc.expected {
					t.Errorf("Expected response %q, got %q", tc.expected, got)
				}
			})
		}
	})
}
