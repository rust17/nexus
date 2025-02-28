package test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cfg "nexus/internal/config"
	"nexus/internal/healthcheck"
	px "nexus/internal/proxy"
	"nexus/internal/route"
)

// benchmarkConfig defines benchmark test configuration
type benchmarkConfig struct {
	Name                string        // Test scenario name
	BalancerType        string        // Load balancer type
	BackendCount        int           // Number of backend servers
	EnableHealthCheck   bool          // Whether to enable health check
	HealthCheckInterval time.Duration // Health check interval
	HealthCheckTimeout  time.Duration // Health check timeout
}

// runProxyBenchmark is a common function to run proxy benchmarks
func runProxyBenchmark(b *testing.B, config benchmarkConfig) {
	// Create test backend servers
	backends := make([]*httptest.Server, config.BackendCount)
	for i := range backends {
		backends[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer backends[i].Close()
	}

	// Create route configuration
	routeConfigs := []*cfg.RouteConfig{
		{
			Name:    "test-route",
			Service: "test-service",
			Match: cfg.RouteMatch{
				Path: "/",
			},
		},
	}

	// Create service configuration
	serviceConfigs := map[string]*cfg.ServiceConfig{
		"test-service": {
			Name:         "test-service",
			BalancerType: config.BalancerType,
			Servers:      make([]cfg.ServerConfig, config.BackendCount),
		},
	}

	// Add backend servers to service config
	for i, backend := range backends {
		serviceConfigs["test-service"].Servers[i] = cfg.ServerConfig{
			Address: backend.URL,
			Weight:  1,
		}
	}

	// Initialize router
	router := route.NewRouter(routeConfigs, serviceConfigs)

	// If health check is enabled, initialize health checker
	var healthChecker *healthcheck.HealthChecker
	if config.EnableHealthCheck {
		healthChecker = healthcheck.NewHealthChecker(
			true,
			config.HealthCheckInterval,
			config.HealthCheckTimeout,
			"health",
		)
		for _, backend := range backends {
			healthChecker.AddServer(backend.URL)
		}
		go healthChecker.Start()
		defer healthChecker.Stop()
	}

	// Initialize reverse proxy
	proxy := px.NewProxy(router)

	// Create test request
	req := httptest.NewRequest("GET", "/", nil)

	b.ReportAllocs() // Report memory allocations
	b.ResetTimer()   // Reset timer

	// Sequential test
	b.Run("Sequential requests", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			proxy.ServeHTTP(w, req)
		}
	})

	// Parallel test
	b.Run("Parallel requests", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				w := httptest.NewRecorder()
				proxy.ServeHTTP(w, req)
			}
		})
	})
}

// BenchmarkProxy tests proxy performance with different load balancing strategies
func BenchmarkProxy(b *testing.B) {
	// Define benchmark scenarios
	benchmarks := []benchmarkConfig{
		// Single backend tests
		{
			Name:              "Single backend_Round Robin",
			BalancerType:      "round_robin",
			BackendCount:      1,
			EnableHealthCheck: false,
		},
		{
			Name:              "Single backend_Weighted Round Robin",
			BalancerType:      "weighted_round_robin",
			BackendCount:      1,
			EnableHealthCheck: false,
		},
		{
			Name:              "Single backend_Least Connections",
			BalancerType:      "least_connections",
			BackendCount:      1,
			EnableHealthCheck: false,
		},

		// Multiple backends tests
		{
			Name:              "Multiple backends_Round Robin",
			BalancerType:      "round_robin",
			BackendCount:      10,
			EnableHealthCheck: false,
		},
		{
			Name:              "Multiple backends_Weighted Round Robin",
			BalancerType:      "weighted_round_robin",
			BackendCount:      10,
			EnableHealthCheck: false,
		},
		{
			Name:              "Multiple backends_Least Connections",
			BalancerType:      "least_connections",
			BackendCount:      10,
			EnableHealthCheck: false,
		},

		// Tests with health check enabled
		{
			Name:                "Health check_Round Robin",
			BalancerType:        "round_robin",
			BackendCount:        1,
			EnableHealthCheck:   true,
			HealthCheckInterval: 10 * time.Second,
			HealthCheckTimeout:  1 * time.Second,
		},
		{
			Name:                "Health check_Weighted Round Robin",
			BalancerType:        "weighted_round_robin",
			BackendCount:        1,
			EnableHealthCheck:   true,
			HealthCheckInterval: 10 * time.Second,
			HealthCheckTimeout:  1 * time.Second,
		},
		{
			Name:                "Health check_Least Connections",
			BalancerType:        "least_connections",
			BackendCount:        1,
			EnableHealthCheck:   true,
			HealthCheckInterval: 10 * time.Second,
			HealthCheckTimeout:  1 * time.Second,
		},
	}

	// Execute all benchmark scenarios
	for _, bm := range benchmarks {
		b.Run(bm.Name, func(b *testing.B) {
			runProxyBenchmark(b, bm)
		})
	}
}

// BenchmarkProxyHighLoad tests proxy performance under high load
func BenchmarkProxyHighLoad(b *testing.B) {
	// High load test configuration
	highLoadTests := []benchmarkConfig{
		{
			Name:                "High load_Round Robin",
			BalancerType:        "round_robin",
			BackendCount:        50, // More backend servers
			EnableHealthCheck:   false,
			HealthCheckInterval: 30 * time.Second,
			HealthCheckTimeout:  2 * time.Second,
		},
		{
			Name:                "High load_Least Connections",
			BalancerType:        "least_connections",
			BackendCount:        50,
			EnableHealthCheck:   false,
			HealthCheckInterval: 30 * time.Second,
			HealthCheckTimeout:  2 * time.Second,
		},
	}

	for _, bm := range highLoadTests {
		b.Run(bm.Name, func(b *testing.B) {
			runProxyBenchmark(b, bm)
		})
	}
}
