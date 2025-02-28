package test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"nexus/internal/config"
	"nexus/internal/healthcheck"
	px "nexus/internal/proxy"
	"nexus/internal/route"
	"nexus/internal/service"
)

// setupTestBackends creates test backend servers
func setupTestBackends() (string, string, func()) {
	// Create first backend server
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Response from backend 1"))
	}))

	// Create second backend server
	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Response from backend 2"))
	}))

	// Return cleanup function
	cleanup := func() {
		backend1.Close()
		backend2.Close()
	}

	return backend1.URL, backend2.URL, cleanup
}

func TestIntegration(t *testing.T) {
	// Create test backend servers
	backend1URL, backend2URL, cleanup := setupTestBackends()
	defer cleanup()

	// Define test cases
	testCases := []struct {
		name          string
		balancerType  string
		servers       []config.ServerConfig
		expectedOrder []string
	}{
		{
			name:         "Round Robin",
			balancerType: "round_robin",
			servers: []config.ServerConfig{
				{Address: backend1URL, Weight: 1},
				{Address: backend2URL, Weight: 1},
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
			servers: []config.ServerConfig{
				{Address: backend1URL, Weight: 2},
				{Address: backend2URL, Weight: 1},
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
			servers: []config.ServerConfig{
				{Address: backend1URL, Weight: 1},
				{Address: backend2URL, Weight: 1},
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
		tc := tc // Avoid closure issues
		t.Run(tc.name, func(t *testing.T) {
			// Create config
			cfg := config.NewConfig()

			// Health check configuration
			cfg.HealthCheck.Interval = 100 * time.Millisecond
			cfg.HealthCheck.Timeout = 1 * time.Second

			// Initialize health checker
			healthChecker := healthcheck.NewHealthChecker(
				true,
				cfg.GetHealthCheckConfig().Interval,
				cfg.GetHealthCheckConfig().Timeout,
				"health",
			)

			for _, server := range tc.servers {
				healthChecker.AddServer(server.Address)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go healthChecker.Start()
			defer healthChecker.Stop()

			// Initialize router
			router := route.NewRouter([]*config.RouteConfig{
				{
					Name:    "test-route",
					Service: "test-service",
					Match: config.RouteMatch{
						Path: "/",
					},
				},
			}, map[string]*config.ServiceConfig{
				"test-service": {
					Name:         "test-service",
					BalancerType: tc.balancerType,
					Servers:      tc.servers,
				},
			})

			// Initialize reverse proxy
			proxy := px.NewProxy(router)

			// Test request routing
			for i, expected := range tc.expectedOrder {
				req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
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

func TestHTTPReverseProxy(t *testing.T) {
	backend1URL, backend2URL, cleanup := setupTestBackends()
	defer cleanup()

	inputConfig := fmt.Sprintf(`
listen_addr: ":8080"
services:
  - name: "test-service1"
    servers:
      - address: %s
  - name: "test-service2"
    servers:
      - address: %s
routes:
  - name: "api-route"
    match:
      path: "*"
      host: "api.example1.com"
    service: "test-service1"
  - name: "api-route2"
    match:
      path: "*"
      host: "api.example2.com"
    service: "test-service2"
`, backend1URL, backend2URL)
	cfgFile := config.CreateTempConfigFile(t, inputConfig)
	defer os.Remove(cfgFile)

	cfg := config.NewConfig()
	cfg.LoadFromFile(cfgFile)

	router := route.NewRouter(cfg.Routes, cfg.Services)
	proxy := px.NewProxy(router)

	ts := httptest.NewServer(proxy)
	defer ts.Close()

	testCases := []struct {
		name     string
		host     string
		expected string
		path     string
	}{
		{
			name:     "api.example1.com root path",
			host:     "api.example1.com",
			expected: "Response from backend 1",
			path:     "/api",
		},
		{
			name:     "api.example1.com test path",
			host:     "api.example1.com",
			expected: "Response from backend 1",
			path:     "/foobar",
		},
		{
			name:     "api.example2.com root path",
			host:     "api.example2.com",
			expected: "Response from backend 2",
			path:     "/api",
		},
		{
			name:     "api.example2.com test path",
			host:     "api.example2.com",
			expected: "Response from backend 2",
			path:     "/barfoo",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil).WithContext(ctx)
			req.Host = tc.host
			w := httptest.NewRecorder()
			proxy.ServeHTTP(w, req)

			got := w.Body.String()
			if got != tc.expected {
				t.Errorf("Test case %s failed: expected %q, got %q", tc.name, tc.expected, got)
			}
		})
	}
}

func TestConfigHotReloadIntegration(t *testing.T) {
	// Create temporary config file
	initialConfig := `
listen_addr: ":8080"
services:
  - name: "test-service"
    balancer_type: "round_robin"
    servers:
      - address: "http://localhost:8081"
        weight: 1
health_check:
  interval: 10s
  timeout: 2s
log_level: "info"
`
	configFile := config.CreateTempConfigFile(t, initialConfig)
	defer os.Remove(configFile)

	// Initialize config watcher
	watcher := config.NewConfigWatcher(configFile)

	var wg sync.WaitGroup
	wg.Add(1)

	// Set config update callback
	watcher.Watch(func(cfg *config.Config) {
		defer wg.Done()

		// Verify updated config
		if cfg.GetListenAddr() != ":8081" {
			t.Errorf("Expected listen address :8081, got %s", cfg.GetListenAddr())
		}

		// Get specific service config
		testService := cfg.Services["test-service"]

		if testService.BalancerType != "weighted_round_robin" {
			t.Errorf("Expected balancer type weighted_round_robin, got %s", testService.BalancerType)
		}

		if len(testService.Servers) != 1 || testService.Servers[0].Address != "http://localhost:8082" {
			t.Errorf("Server list doesn't match expected: %v", testService.Servers)
		}

		if cfg.GetHealthCheckConfig().Interval != 5*time.Second {
			t.Errorf("Expected health check interval 5s, got %v", cfg.GetHealthCheckConfig().Interval)
		}
		if cfg.GetHealthCheckConfig().Timeout != 1*time.Second {
			t.Errorf("Expected health check timeout 1s, got %v", cfg.GetHealthCheckConfig().Timeout)
		}
		if cfg.GetLogLevel() != "debug" {
			t.Errorf("Expected log level debug, got %s", cfg.GetLogLevel())
		}
	})

	// Start watcher
	go watcher.Start()

	// Update config file
	updatedConfig := `
listen_addr: ":8081"
services:
  - name: "test-service"
    balancer_type: "weighted_round_robin"
    servers:
      - address: "http://localhost:8082"
        weight: 2
health_check:
  interval: 5s
  timeout: 1s
log_level: "debug"
`
	// Write new config
	if err := os.WriteFile(configFile, []byte(updatedConfig), 0644); err != nil {
		t.Fatalf("Failed to update config file: %v", err)
	}

	// Wait for watcher initialization
	time.Sleep(500 * time.Millisecond)

	// Use channel and timeout mechanism to wait for config update
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Config successfully updated
	case <-time.After(3 * time.Second):
		t.Fatal("Config update timeout, no update event detected")
	}
}

// TestServiceIntegration tests service integration functionality
func TestServiceIntegration(t *testing.T) {
	// Create test backend servers
	backend1URL, backend2URL, cleanup := setupTestBackends()
	defer cleanup()

	// Create service config
	svcConfig := &config.ServiceConfig{
		Name:         "api-service",
		BalancerType: "round_robin",
		Servers: []config.ServerConfig{
			{Address: backend1URL, Weight: 1},
			{Address: backend2URL, Weight: 1},
		},
	}

	// Create service instance
	svc := service.NewService(svcConfig)

	// Verify service name
	if svc.Name() != "api-service" {
		t.Errorf("Expected service name api-service, got %s", svc.Name())
	}

	// Test load balancing
	ctx := context.Background()

	// Verify server rotation
	server1, err := svc.NextServer(ctx)
	if err != nil {
		t.Fatalf("Failed to get next server: %v", err)
	}

	server2, err := svc.NextServer(ctx)
	if err != nil {
		t.Fatalf("Failed to get next server: %v", err)
	}

	if server1 == server2 {
		t.Error("Round Robin load balancing should return different servers")
	}

	// Test service update
	updatedConfig := &config.ServiceConfig{
		Name:         "api-service-updated",
		BalancerType: "weighted_round_robin",
		Servers: []config.ServerConfig{
			{Address: backend1URL, Weight: 2},
			{Address: backend2URL, Weight: 1},
		},
	}

	if err := svc.Update(updatedConfig); err != nil {
		t.Fatalf("Failed to update service config: %v", err)
	}

	// Verify updated config
	if svc.Name() != "api-service-updated" {
		t.Errorf("Updated service name should be api-service-updated, got %s", svc.Name())
	}
}
