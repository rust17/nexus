package test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nexus/internal"
	lb "nexus/internal/balancer"
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

	// Define test cases for different load balancing algorithms
	testCases := []struct {
		name          string
		balancerType  string
		servers       []internal.ServerConfig
		expectedOrder []string
	}{
		{
			name:         "Round Robin",
			balancerType: "round_robin",
			servers: []internal.ServerConfig{
				{Address: backend1.URL, Weight: 1},
				{Address: backend2.URL, Weight: 1},
			},
			expectedOrder: []string{
				"Response from backend 1",
				"Response from backend 2",
				"Response from backend 1",
				"Response from backend 2",
			},
		},
		{
			name:         "Weighted Round Robin",
			balancerType: "weighted_round_robin",
			servers: []internal.ServerConfig{
				{Address: backend1.URL, Weight: 2},
				{Address: backend2.URL, Weight: 1},
			},
			expectedOrder: []string{
				"Response from backend 1",
				"Response from backend 1",
				"Response from backend 2",
				"Response from backend 1",
				"Response from backend 1",
				"Response from backend 2",
			},
		},
		{
			name:         "Least Connections",
			balancerType: "least_connections",
			servers: []internal.ServerConfig{
				{Address: backend1.URL, Weight: 1},
				{Address: backend2.URL, Weight: 1},
			},
			expectedOrder: []string{
				"Response from backend 1",
				"Response from backend 2",
				"Response from backend 1",
				"Response from backend 2",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc // Prevent closure issues
		t.Run(tc.name, func(t *testing.T) {
			// Create configuration
			cfg := internal.NewConfig()
			cfg.BalancerType = tc.balancerType
			cfg.Servers = tc.servers
			cfg.HealthCheck.Interval = 100 * time.Millisecond
			cfg.HealthCheck.Timeout = 1 * time.Second

			// Initialize load balancer
			balancer := lb.NewBalancer(tc.balancerType)
			for _, server := range cfg.GetServers() {
				if cfg.GetBalancerType() == "weighted_round_robin" {
					if wrr, ok := balancer.(*lb.WeightedRoundRobinBalancer); ok {
						wrr.AddWithWeight(server.Address, server.Weight)
					}
				} else {
					balancer.Add(server.Address)
				}
			}

			// Initialize health checker
			healthChecker := internal.NewHealthChecker(cfg.GetHealthCheckConfig().Interval, cfg.GetHealthCheckConfig().Timeout)
			for _, server := range cfg.GetServers() {
				healthChecker.AddServer(server.Address)
			}
			go healthChecker.Start()
			defer healthChecker.Stop()

			// Initialize reverse proxy
			proxy := internal.NewProxy(balancer)

			// Test request routing
			req := httptest.NewRequest("GET", "/", nil)
			for i, expected := range tc.expectedOrder {
				w := httptest.NewRecorder()
				proxy.ServeHTTP(w, req)

				got := w.Body.String()
				if got != expected {
					t.Errorf("Test case %s, iteration %d failed: expected %q, got %q", tc.name, i, expected, got)
				}
			}
		})
	}
}
