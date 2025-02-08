package test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	lb "nexus/internal/balancer"
	"nexus/internal/config"
	"nexus/internal/healthcheck"
	px "nexus/internal/proxy"
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
		servers       []config.ServerConfig
		expectedOrder []string
	}{
		{
			name:         "Round Robin",
			balancerType: "round_robin",
			servers: []config.ServerConfig{
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
			servers: []config.ServerConfig{
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
			servers: []config.ServerConfig{
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
			cfg := config.NewConfig()
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
			healthChecker := healthcheck.NewHealthChecker(cfg.GetHealthCheckConfig().Interval, cfg.GetHealthCheckConfig().Timeout)
			for _, server := range cfg.GetServers() {
				healthChecker.AddServer(server.Address)
			}
			go healthChecker.Start()
			defer healthChecker.Stop()

			// Initialize reverse proxy
			proxy := px.NewProxy(balancer)

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

func TestConfigHotReloadIntegration(t *testing.T) {
	// create a temp config file
	configContent := `
listen_addr: ":8080"
balancer_type: "round_robin"
servers:
  - address: "http://localhost:8081"
    weight: 1
health_check:
  interval: 10s
  timeout: 2s
log_level: "info"
`
	configFile := config.CreateTempConfigFile(t, configContent)
	defer os.Remove(configFile)

	// init config watcher
	watcher := config.NewConfigWatcher(configFile)

	go watcher.Start()

	// update config file
	newConfigContent := `
listen_addr: ":8081"
balancer_type: "weighted_round_robin"
servers:
  - address: "http://localhost:8082"
    weight: 2
health_check:
  interval: 5s
  timeout: 1s
log_level: "debug"
`
	time.Sleep(2 * time.Second)
	if err := os.WriteFile(configFile, []byte(newConfigContent), 0644); err != nil {
		t.Fatalf("Failed to update config file: %v", err)
	}

	var updated bool
	watcher.Watch(func(cfg *config.Config) {
		updated = true
		// verify updated config
		if cfg.GetListenAddr() != ":8081" {
			t.Errorf("Expected listen_addr :8081, got %s", cfg.GetListenAddr())
		}
		if cfg.GetBalancerType() != "weighted_round_robin" {
			t.Errorf("Expected balancer_type weighted_round_robin, got %s", cfg.GetBalancerType())
		}
		if len(cfg.GetServers()) != 1 || cfg.GetServers()[0].Address != "http://localhost:8082" {
			t.Errorf("Unexpected servers list: %v", cfg.GetServers())
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

	// wait for update
	time.Sleep(2 * time.Second)
	if !updated {
		t.Error("Config update not detected")
	}
}
