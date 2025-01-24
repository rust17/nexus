package test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nexus/internal"
)

const (
	healthCheckInterval = 100 * time.Millisecond
	healthCheckTimeout  = 1 * time.Second
	pollInterval        = 500 * time.Millisecond
	pollCount           = 10
)

func TestHealthChecker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		serverHandler http.HandlerFunc
		expectHealthy bool
	}{
		{
			name: "HealthyServer",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectHealthy: true,
		},
		{
			name: "UnhealthyServer",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectHealthy: false,
		},
		{
			name: "TimeoutServer",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(2 * time.Second)
				w.WriteHeader(http.StatusOK)
			},
			expectHealthy: false,
		},
	}

	for _, tt := range tests {
		tt := tt // Prevent closure issues
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test server
			ts := httptest.NewServer(tt.serverHandler)
			defer ts.Close()

			// Create health checker
			checker := internal.NewHealthChecker(healthCheckInterval, healthCheckTimeout)
			checker.AddServer(ts.URL)
			go checker.Start()
			defer checker.Stop()

			// Wait for health check results using polling
			var healthy bool
			for i := 0; i < pollCount; i++ {
				if checker.IsHealthy(ts.URL) == tt.expectHealthy {
					healthy = true
					break
				}
				time.Sleep(pollInterval)
			}

			if !healthy {
				t.Errorf("Expected server to be healthy=%v, but got %v", tt.expectHealthy, !tt.expectHealthy)
			}
		})
	}
}

func TestHealthChecker_RemoveServer(t *testing.T) {
	t.Parallel()

	// Create test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Create health checker
	checker := internal.NewHealthChecker(healthCheckInterval, healthCheckTimeout)
	checker.AddServer(ts.URL)
	go checker.Start()
	defer checker.Stop()

	// Wait for health check execution
	time.Sleep(200 * time.Millisecond)

	// Remove server
	checker.RemoveServer(ts.URL)
	if checker.IsHealthy(ts.URL) {
		t.Error("Removed server should not be considered healthy")
	}
}
