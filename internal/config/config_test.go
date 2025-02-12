package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigLoad_ValidConfig(t *testing.T) {
	t.Parallel()

	configContent := `
listen_addr: ":8080"
services:
  - name: "web-service"
    balancer_type: "round_robin"
    servers:
      - address: "http://backend1:8080"
      - address: "http://backend2:8080"
  - name: "api-service"
    balancer_type: "weighted_round_robin"
    servers:
      - address: "http://api1:8080"
        weight: 3
      - address: "http://api2:8080"
        weight: 2
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

	if cfg.GetBalancerType("web-service") != "round_robin" {
		t.Errorf("Expected balancer_type round_robin, got %s", cfg.GetBalancerType("web-service"))
	}

	if cfg.GetBalancerType("api-service") != "weighted_round_robin" {
		t.Errorf("Expected balancer_type weighted_round_robin, got %s", cfg.GetBalancerType("api-service"))
	}

	servers := cfg.GetServers("web-service")
	if len(servers) != 2 || servers[0].Address != "http://backend1:8080" || servers[1].Address != "http://backend2:8080" {
		t.Errorf("Unexpected servers list: %v", servers)
	}

	servers = cfg.GetServers("api-service")
	if len(servers) != 2 || servers[0].Address != "http://api1:8080" || servers[1].Address != "http://api2:8080" {
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
services:
  - name: "web-service"
    balancer_type: "round_robin"
    servers:
      - address: "http://backend1:8080"
      - address: "http://backend2:8080"
health_check:
  interval: 10s
  timeout: 2s
log_level: "info"
`
	configFile := createTempConfigFile(t, configContent)
	defer os.Remove(configFile)

	// 初始化配置监控器
	watcher := NewConfigWatcher(configFile)

	// 使用原子操作
	var updated int32
	watcher.Watch(func(cfg *Config) {
		atomic.StoreInt32(&updated, 1) // 使用原子存储
	})

	go watcher.Start()

	// 修改配置文件
	newConfigContent := `
listen_addr: ":8081"
services:
  - name: "api-service"
    balancer_type: "weighted_round_robin"
    servers:
      - address: "http://api1:8080"
        weight: 3
      - address: "http://api2:8080"
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
	if atomic.LoadInt32(&updated) == 0 {
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
services:
  - name: "web-service"
    balancer_type: "round_robin"
    servers:
      - address: "http://backend1:8080"
      - address: "http://backend2:8080"
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
services:
  - name: "web-service"
    balancer_type: "invalid_type"
    servers:
      - address: "http://backend1:8080"
      - address: "http://backend2:8080"
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
services:
  - name: "web-service"
    balancer_type: "round_robin"
health_check:
  interval: 10s
  timeout: 2s
`,
			expectedErr: "server list cannot be empty",
		},
		{
			name: "EmptyServerAddress",
			config: `
listen_addr: ":8080"
services:
  - name: "web-service"
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
services:
  - name: "web-service"
    balancer_type: "round_robin"
    servers:
      - address: "http://backend1:8080"
      - address: "http://backend2:8080"
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
services:
  - name: "web-service"
    balancer_type: "round_robin"
    servers:
      - address: "http://backend1:8080"
      - address: "http://backend2:8080"
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
services:
  - name: "web-service"
    balancer_type: "round_robin"
    servers:
      - address: "http://backend1:8080"
      - address: "http://backend2:8080"
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
services:
  - name: "web-service"
    balancer_type: "round_robin"
    servers:
      - address: "http://backend1:8080"
      - address: "http://backend2:8080"
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
services:
  - name: "api-service"
    balancer_type: "weighted_round_robin"
    servers:
      - address: "http://api1:8080"
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
	cfg.Services = make(map[string]*ServiceConfig)
	cfg.Services["web-service"] = &ServiceConfig{} // 初始化服务配置

	// 初始值测试
	if cfg.GetBalancerType("web-service") != "" {
		t.Errorf("Expected empty balancer type, got %s", cfg.GetBalancerType("web-service"))
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
				if err := cfg.UpdateBalancerType("web-service", tc.input); err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if actual := cfg.GetBalancerType("web-service"); actual != tc.expected {
					t.Errorf("Expected %s, got %s", tc.expected, actual)
				}
			})
		}
	})

	// 无效类型测试
	t.Run("InvalidType", func(t *testing.T) {
		err := cfg.UpdateBalancerType("web-service", "invalid_type")
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
		cfg.Services = make(map[string]*ServiceConfig)
		cfg.Services["web-service"] = &ServiceConfig{} // 初始化服务配置

		for i := 0; i < total; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				bType := types[index%3]
				cfg.UpdateBalancerType("web-service", bType)
			}(i)
		}

		wg.Wait()

		// 验证最终结果
		var updateCount int
		for i := 0; i < total; i++ {
			if cfg.GetBalancerType("web-service") == types[i%3] {
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
	cfg.Services = make(map[string]*ServiceConfig)
	cfg.Services["web-service"] = &ServiceConfig{
		BalancerType: "round_robin",
	} // 初始化服务配置

	// 初始值测试
	if len(cfg.GetServers("web-service")) != 0 {
		t.Errorf("Expected empty servers, got %v", cfg.GetServers("web-service"))
	}

	// 正常更新测试
	t.Run("ValidServers", func(t *testing.T) {
		validServers := []ServerConfig{
			{Address: "http://server1", Weight: 1},
			{Address: "http://server2", Weight: 1},
		}

		if err := cfg.UpdateServers("web-service", validServers); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if actual := cfg.GetServers("web-service"); !reflect.DeepEqual(actual, validServers) {
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
				setup:       func() { cfg.UpdateBalancerType("web-service", "weighted_round_robin") },
				expectedErr: "invalid weight for server http://server1: 0",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.setup != nil {
					tc.setup()
				}
				err := cfg.UpdateServers("web-service", tc.servers)
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
				cfg.UpdateServers("web-service", servers)
				_ = cfg.GetServers("web-service")
			}(i)
		}
		wg.Wait()

		// 验证最终一致性
		finalServers := cfg.GetServers("web-service")
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

func TestUpdateLogLevel(t *testing.T) {
	t.Parallel()

	cfg := NewConfig()

	// 初始值测试
	if cfg.GetLogLevel() != "" {
		t.Errorf("Expected empty log level, got %s", cfg.GetLogLevel())
	}

	// 正常更新测试
	t.Run("ValidLevels", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"debug", "debug"},
			{"info", "info"},
			{"warn", "warn"},
			{"error", "error"},
			{"fatal", "fatal"},
			{"", ""},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				if err := cfg.UpdateLogLevel(tc.input); err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if actual := cfg.GetLogLevel(); actual != tc.expected {
					t.Errorf("Expected %s, got %s", tc.expected, actual)
				}
			})
		}
	})

	// 无效级别测试
	t.Run("InvalidLevel", func(t *testing.T) {
		err := cfg.UpdateLogLevel("invalid")
		if err == nil {
			t.Fatal("Expected error but got nil")
		}
		expectedErr := "invalid log level"
		if !strings.Contains(err.Error(), expectedErr) {
			t.Errorf("Expected error to contain %q, got %q", expectedErr, err.Error())
		}
	})

	// 并发更新测试
	t.Run("ConcurrentUpdates", func(t *testing.T) {
		var wg sync.WaitGroup
		levels := []string{"debug", "info", "warn", "error", "fatal"}
		total := 100

		// 重置配置确保测试独立性
		cfg = NewConfig()

		for i := 0; i < total; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				level := levels[index%5]
				cfg.UpdateLogLevel(level)
			}(i)
		}

		wg.Wait()

		// 验证最终结果合法性
		finalLevel := cfg.GetLogLevel()
		if finalLevel != "" {
			valid := false
			for _, l := range levels {
				if finalLevel == l {
					valid = true
					break
				}
			}
			if !valid {
				t.Errorf("Unexpected final log level: %s", finalLevel)
			}
		}
	})
}

// 新增OpenTelemetry配置相关测试
func TestTelemetryConfig(t *testing.T) {
	// 测试用例
	testCases := []struct {
		name     string
		config   string
		expected TelemetryConfig
	}{
		{
			name: "default_opentelemetry",
			config: `
telemetry:
  opentelemetry:
    enabled: true
    endpoint: "localhost:4317"
    service_name: "nexus-service"
    metrics:
      interval: 30s`,
			expected: TelemetryConfig{
				OpenTelemetry: OpenTelemetryConfig{
					Enabled:     true,
					Endpoint:    "localhost:4317",
					ServiceName: "nexus-service",
					Metrics: MetricConfig{
						Interval: 30 * time.Second,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfig()
			tmpFile := createTempConfigFile(t, tc.config)
			require.NoError(t, cfg.LoadFromFile(tmpFile))

			// 验证配置解析
			telemetryCfg := cfg.GetTelemetryConfig()
			assert.Equal(t, tc.expected.OpenTelemetry.Enabled, telemetryCfg.OpenTelemetry.Enabled)
			assert.Equal(t, tc.expected.OpenTelemetry.Endpoint, telemetryCfg.OpenTelemetry.Endpoint)
			assert.Equal(t, tc.expected.OpenTelemetry.ServiceName, telemetryCfg.OpenTelemetry.ServiceName)
			assert.Equal(t, tc.expected.OpenTelemetry.Metrics.Interval, telemetryCfg.OpenTelemetry.Metrics.Interval)
		})
	}

	// 测试并发读取
	t.Run("concurrent_access", func(t *testing.T) {
		cfg := NewConfig()
		tmpFile := createTempConfigFile(t, testCases[0].config)
		require.NoError(t, cfg.LoadFromFile(tmpFile))

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				telemetryCfg := cfg.GetTelemetryConfig()
				assert.True(t, telemetryCfg.OpenTelemetry.Enabled)
			}()
		}
		wg.Wait()
	})
}
