package config

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestConfigLoad_ValidConfig(t *testing.T) {
	t.Parallel()

	configContent := `
listen_addr: ":8080"
balancer_type: "round_robin"
servers:
  - address: "http://localhost:8081"
    weight: 1
  - address: "http://localhost:8082"
    weight: 1
health_check:
  interval: 10s
  timeout: 2s
log_level: "debug"
`

	configFile := createTempConfigFile(t, configContent)

	cfg := NewConfig()
	if err := cfg.LoadFromFile(configFile); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.GetListenAddr() != ":8080" {
		t.Errorf("Expected listen_addr :8080, got %s", cfg.GetListenAddr())
	}

	if cfg.GetBalancerType() != "round_robin" {
		t.Errorf("Expected balancer_type round_robin, got %s", cfg.GetBalancerType())
	}

	servers := cfg.GetServers()
	if len(servers) != 2 || servers[0].Address != "http://localhost:8081" || servers[1].Address != "http://localhost:8082" {
		t.Errorf("Unexpected servers list: %v", servers)
	}

	healthCheckCfg := cfg.GetHealthCheckConfig()
	if healthCheckCfg.Interval != 10*time.Second {
		t.Errorf("Expected health check interval 10s, got %v", healthCheckCfg.Interval)
	}

	if healthCheckCfg.Timeout != 2*time.Second {
		t.Errorf("Expected health check timeout 2s, got %v", healthCheckCfg.Timeout)
	}

	if cfg.GetLogLevel() != "debug" {
		t.Errorf("Expected log level debug, got %s", cfg.GetLogLevel())
	}
}

func TestConfigLoad_InvalidFile(t *testing.T) {
	t.Parallel()

	cfg := NewConfig()
	err := cfg.LoadFromFile("nonexistent.yaml")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Expected os.ErrNotExist, got %v", err)
	}
}

func TestInvalidConfigFormat(t *testing.T) {
	// Create invalid configuration file
	tmpFile, err := os.CreateTemp("", "config-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	cfg := NewConfig()
	err = cfg.LoadFromFile(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for invalid config format")
	}
}

func TestConfigHotReload(t *testing.T) {
	// 创建临时配置文件
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
	configFile := createTempConfigFile(t, configContent)
	defer os.Remove(configFile)

	// 初始化配置监控器
	watcher := NewConfigWatcher(configFile)

	var updated bool
	watcher.Watch(func(cfg *Config) {
		updated = true
	})

	go watcher.Start()

	// 修改配置文件
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
	if err := os.WriteFile(configFile, []byte(newConfigContent), 0644); err != nil {
		t.Fatalf("Failed to update config file: %v", err)
	}

	// 等待更新
	time.Sleep(1 * time.Second)
	if !updated {
		t.Error("Config update not detected")
	}
}

func TestConfigLoad_InValidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      string
		expectedErr string
	}{
		{
			name: "EmptyListenAddr",
			config: `
balancer_type: "round_robin"
servers:
  - address: "http://localhost:8081"
health_check:
  interval: 10s
  timeout: 2s
`,
			expectedErr: "listen address cannot be empty",
		},
		{
			name: "InvalidBalancerType",
			config: `
listen_addr: ":8080"
balancer_type: "invalid_type"
servers:
  - address: "http://localhost:8081"
health_check:
  interval: 10s
  timeout: 2s
`,
			expectedErr: "invalid balancer type",
		},
		{
			name: "EmptyServerList",
			config: `
listen_addr: ":8080"
balancer_type: "round_robin"
health_check:
  interval: 10s
  timeout: 2s
`,
			expectedErr: "at least one server must be configured",
		},
		{
			name: "EmptyServerAddress",
			config: `
listen_addr: ":8080"
balancer_type: "round_robin"
servers:
  - address: ""
health_check:
  interval: 10s
  timeout: 2s
`,
			expectedErr: "server address cannot be empty",
		},
		{
			name: "InvalidHealthCheckInterval",
			config: `
listen_addr: ":8080"
balancer_type: "round_robin"
servers:
  - address: "http://localhost:8081"
health_check:
  interval: 0s
  timeout: 2s
`,
			expectedErr: "health check interval must be positive",
		},
		{
			name: "InvalidHealthCheckTimeout",
			config: `
listen_addr: ":8080"
balancer_type: "round_robin"
servers:
  - address: "http://localhost:8081"
health_check:
  interval: 10s
  timeout: 0s
`,
			expectedErr: "health check timeout must be positive",
		},
		{
			name: "TimeoutGreaterThanInterval",
			config: `
listen_addr: ":8080"
balancer_type: "round_robin"
servers:
  - address: "http://localhost:8081"
health_check:
  interval: 5s
  timeout: 10s
`,
			expectedErr: "health check timeout must be less than interval",
		},
		{
			name: "InvalidLogLevel",
			config: `
listen_addr: ":8080"
balancer_type: "round_robin"
servers:
  - address: "http://localhost:8081"
health_check:
  interval: 10s
  timeout: 2s
log_level: "invalid_level"
`,
			expectedErr: "invalid log level",
		},
		{
			name: "InvalidWeightedRoundRobin",
			config: `
listen_addr: ":8080"
balancer_type: "weighted_round_robin"
servers:
  - address: "http://localhost:8081"
    weight: 0
health_check:
  interval: 10s
  timeout: 2s
`,
			expectedErr: "invalid weight for server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := createTempConfigFile(t, tt.config)

			err := Validate(configFile)
			if err == nil {
				t.Fatal("Expected error but got nil")
			}

			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("Expected error to contain %q, got %q", tt.expectedErr, err.Error())
			}
		})
	}
}
