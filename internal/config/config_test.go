package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
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

func TestUpdateListenAddr(t *testing.T) {
	t.Parallel()

	cfg := NewConfig()

	// 初始值测试
	if cfg.GetListenAddr() != "" {
		t.Errorf("Expected empty listen address, got %s", cfg.GetListenAddr())
	}

	// 正常更新测试
	expectedAddr := ":8081"
	cfg.UpdateListenAddr(expectedAddr)
	if actual := cfg.GetListenAddr(); actual != expectedAddr {
		t.Errorf("Expected %s, got %s", expectedAddr, actual)
	}

	// 并发更新测试
	var wg sync.WaitGroup
	total := 100
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			cfg.UpdateListenAddr(addr)
			_ = cfg.GetListenAddr()
		}(fmt.Sprintf(":%d", 8080+i))
	}
	wg.Wait()

	// 验证最终结果
	var updateCount int
	for i := 0; i < total; i++ {
		if cfg.GetListenAddr() == fmt.Sprintf(":%d", 8080+i) {
			updateCount++
		}
	}
	if updateCount == 0 {
		t.Error("No successful updates detected")
	}
}

func TestUpdateBalancerType(t *testing.T) {
	t.Parallel()

	cfg := NewConfig()

	// 初始值测试
	if cfg.GetBalancerType() != "" {
		t.Errorf("Expected empty balancer type, got %s", cfg.GetBalancerType())
	}

	// 正常更新测试
	t.Run("ValidTypes", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"round_robin", "round_robin"},
			{"weighted_round_robin", "weighted_round_robin"},
			{"least_connections", "least_connections"},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				if err := cfg.UpdateBalancerType(tc.input); err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if actual := cfg.GetBalancerType(); actual != tc.expected {
					t.Errorf("Expected %s, got %s", tc.expected, actual)
				}
			})
		}
	})

	// 无效类型测试
	t.Run("InvalidType", func(t *testing.T) {
		err := cfg.UpdateBalancerType("invalid_type")
		if err == nil {
			t.Fatal("Expected error but got nil")
		}
		expectedErr := "invalid balancer type"
		if !strings.Contains(err.Error(), expectedErr) {
			t.Errorf("Expected error to contain %q, got %q", expectedErr, err.Error())
		}
	})

	// 并发更新测试
	t.Run("ConcurrentUpdates", func(t *testing.T) {
		var wg sync.WaitGroup
		types := []string{"round_robin", "weighted_round_robin", "least_connections"}
		total := 100

		// 重置配置确保测试独立性
		cfg = NewConfig()

		for i := 0; i < total; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				bType := types[index%3]
				cfg.UpdateBalancerType(bType)
			}(i)
		}

		wg.Wait()

		// 验证最终结果
		var updateCount int
		for i := 0; i < total; i++ {
			if cfg.GetBalancerType() == types[i%3] {
				updateCount++
			}
		}
		if updateCount == 0 {
			t.Error("No successful updates detected")
		}
	})
}

func TestUpdateServers(t *testing.T) {
	t.Parallel()

	cfg := NewConfig()
	cfg.UpdateBalancerType("round_robin") // 初始化负载均衡类型

	// 初始值测试
	if len(cfg.GetServers()) != 0 {
		t.Errorf("Expected empty servers, got %v", cfg.GetServers())
	}

	// 正常更新测试
	t.Run("ValidServers", func(t *testing.T) {
		validServers := []ServerConfig{
			{Address: "http://server1", Weight: 1},
			{Address: "http://server2", Weight: 1},
		}

		if err := cfg.UpdateServers(validServers); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if actual := cfg.GetServers(); !reflect.DeepEqual(actual, validServers) {
			t.Errorf("Expected %v, got %v", validServers, actual)
		}
	})

	// 异常情况测试
	t.Run("InvalidCases", func(t *testing.T) {
		testCases := []struct {
			name        string
			servers     []ServerConfig
			expectedErr string
			setup       func()
		}{
			{
				name:        "EmptyList",
				servers:     []ServerConfig{},
				expectedErr: "server list cannot be empty",
			},
			{
				name: "EmptyAddress",
				servers: []ServerConfig{
					{Address: "", Weight: 1},
				},
				expectedErr: "server address cannot be empty",
			},
			{
				name: "InvalidWeightForWRR",
				servers: []ServerConfig{
					{Address: "http://server1", Weight: 0},
				},
				setup:       func() { cfg.UpdateBalancerType("weighted_round_robin") },
				expectedErr: "invalid weight 0",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.setup != nil {
					tc.setup()
				}
				err := cfg.UpdateServers(tc.servers)
				if err == nil || !strings.Contains(err.Error(), tc.expectedErr) {
					t.Errorf("Expected error containing %q, got %v", tc.expectedErr, err)
				}
			})
		}
	})

	// 并发更新测试
	t.Run("ConcurrentUpdates", func(t *testing.T) {
		var wg sync.WaitGroup
		testServers := [][]ServerConfig{
			{{Address: "http://serverA", Weight: 1}},
			{{Address: "http://serverB", Weight: 2}},
			{{Address: "http://serverC", Weight: 3}},
		}

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				servers := testServers[index%3]
				cfg.UpdateServers(servers)
				_ = cfg.GetServers()
			}(i)
		}
		wg.Wait()

		// 验证最终一致性
		finalServers := cfg.GetServers()
		valid := false
		for _, s := range testServers {
			if reflect.DeepEqual(finalServers, s) {
				valid = true
				break
			}
		}
		if !valid {
			t.Errorf("Unexpected final server list: %v", finalServers)
		}
	})
}

func TestUpdateHealthCheck(t *testing.T) {
	t.Parallel()

	cfg := NewConfig()

	// 初始值测试
	initialHC := cfg.GetHealthCheckConfig()
	if initialHC.Interval != 0 || initialHC.Timeout != 0 {
		t.Errorf("Expected empty health check config, got %+v", initialHC)
	}

	// 正常更新测试
	t.Run("ValidConfig", func(t *testing.T) {
		testCases := []struct {
			interval time.Duration
			timeout  time.Duration
		}{
			{10 * time.Second, 2 * time.Second},
			{5 * time.Minute, 1 * time.Minute},
			{30 * time.Second, 5 * time.Second},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%v/%v", tc.interval, tc.timeout), func(t *testing.T) {
				if err := cfg.UpdateHealthCheck(tc.interval, tc.timeout); err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				hc := cfg.GetHealthCheckConfig()
				if hc.Interval != tc.interval || hc.Timeout != tc.timeout {
					t.Errorf("Expected %v/%v, got %v/%v",
						tc.interval, tc.timeout, hc.Interval, hc.Timeout)
				}
			})
		}
	})

	// 异常情况测试
	t.Run("InvalidConfig", func(t *testing.T) {
		testCases := []struct {
			name     string
			interval time.Duration
			timeout  time.Duration
			wantErr  string
		}{
			{"NegativeInterval", -time.Second, 2 * time.Second, "interval must be positive"},
			{"ZeroTimeout", 5 * time.Second, 0, "timeout must be positive"},
			{"TimeoutTooLarge", 5 * time.Second, 10 * time.Second, "timeout must be less"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := cfg.UpdateHealthCheck(tc.interval, tc.timeout)
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("Expected error containing %q, got %v", tc.wantErr, err)
				}
			})
		}
	})

	// 并发更新测试
	t.Run("ConcurrentUpdates", func(t *testing.T) {
		var wg sync.WaitGroup
		testCases := []struct {
			interval time.Duration
			timeout  time.Duration
		}{
			{10 * time.Second, 2 * time.Second},
			{5 * time.Second, 1 * time.Second},
			{30 * time.Second, 5 * time.Second},
		}

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				tc := testCases[index%3]
				cfg.UpdateHealthCheck(tc.interval, tc.timeout)
				_ = cfg.GetHealthCheckConfig()
			}(i)
		}
		wg.Wait()

		// 验证最终一致性
		finalHC := cfg.GetHealthCheckConfig()
		valid := false
		for _, tc := range testCases {
			if finalHC.Interval == tc.interval && finalHC.Timeout == tc.timeout {
				valid = true
				break
			}
		}
		if !valid {
			t.Errorf("Unexpected final health check config: %+v", finalHC)
		}
	})
}
