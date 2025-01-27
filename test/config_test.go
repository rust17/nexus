package test

import (
	"errors"
	"os"
	"testing"
	"time"

	"nexus/internal/config"
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

	cfg := config.NewConfig()
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

	cfg := config.NewConfig()
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

	cfg := config.NewConfig()
	err = cfg.LoadFromFile(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for invalid config format")
	}
}

func createTempConfigFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	return tmpFile.Name()
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
	watcher := config.NewConfigWatcher(configFile)

	var updated bool
	watcher.Watch(func(cfg *config.Config) {
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
