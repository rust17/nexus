package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	lg "nexus/internal/logger"

	"gopkg.in/yaml.v3"
)

// Config struct contains all configuration items
type Config struct {
	mu sync.RWMutex

	// Server configuration
	ListenAddr string `yaml:"listen_addr" json:"listen_addr"`

	// Log configuration
	LogLevel string `yaml:"log_level" json:"log_level"`

	// Telemetry configuration
	Telemetry TelemetryConfig `yaml:"telemetry" json:"telemetry"`

	// 服务列表
	Services map[string]*ServiceConfig `yaml:"services" json:"services"`

	HealthCheck HealthCheckConfig `yaml:"health_check" json:"health_check"`
}

// ServerConfig represents a server with its weight
type ServerConfig struct {
	Address string `yaml:"address" json:"address"`
	Weight  int    `yaml:"weight" json:"weight"`
}

// HealthCheckConfig health check configuration
type HealthCheckConfig struct {
	Interval time.Duration `yaml:"interval" json:"interval"`
	Timeout  time.Duration `yaml:"timeout" json:"timeout"`
	Protocol string        `yaml:"protocol" json:"protocol"`
}

// TelemetryConfig telemetry configuration
type TelemetryConfig struct {
	OpenTelemetry OpenTelemetryConfig `yaml:"opentelemetry" json:"opentelemetry"`
}

// OpenTelemetryConfig OpenTelemetry configuration
type OpenTelemetryConfig struct {
	Enabled     bool         `yaml:"enabled" json:"enabled"`
	Endpoint    string       `yaml:"endpoint" json:"endpoint"`
	ServiceName string       `yaml:"service_name" json:"service_name"`
	Metrics     MetricConfig `yaml:"metrics" json:"metrics"`
}

// MetricConfig metric configuration
type MetricConfig struct {
	Interval time.Duration `yaml:"interval" json:"interval"`
}

// ConfigWatcher struct for file monitoring
type ConfigWatcher struct {
	mu       sync.RWMutex
	filePath string
	lastMod  time.Time
	watchers []func(*Config)
}

// NewConfig creates a new configuration instance
func NewConfig() *Config {
	return &Config{}
}

// LoadFromFile loads configuration from a file
func (c *Config) LoadFromFile(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Decide whether to use YAML or JSON based on the file extension
	switch filepath.Ext(path) {
	case ".yaml", ".yml":
		return yaml.Unmarshal(data, c)
	case ".json":
		return json.Unmarshal(data, c)
	default:
		return errors.New("unsupported config file format")
	}
}

// SaveToFile saves configuration to a file
func (c *Config) SaveToFile(path string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var data []byte
	var err error

	// Decide whether to use YAML or JSON based on the file extension
	switch filepath.Ext(path) {
	case ".yaml", ".yml":
		data, err = yaml.Marshal(c)
	case ".json":
		data, err = json.MarshalIndent(c, "", "  ")
	default:
		return errors.New("unsupported config file format")
	}

	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetListenAddr gets the listening address
func (c *Config) GetListenAddr() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.ListenAddr
}

// GetBalancerType gets the load balancer type
func (c *Config) GetBalancerType(serviceName string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sConfig, ok := c.Services[serviceName]
	if !ok {
		return ""
	}

	return sConfig.BalancerType
}

// GetServers gets the server list
func (c *Config) GetServers(serviceName string) []ServerConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sConfig, ok := c.Services[serviceName]
	if !ok {
		return nil
	}

	return sConfig.Servers
}

// GetHealthCheckConfig gets the health check configuration
func (c *Config) GetHealthCheckConfig() HealthCheckConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.HealthCheck
}

// GetLogLevel gets the log level
func (c *Config) GetLogLevel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.LogLevel
}

// GetTelemetryConfig gets the telemetry configuration
func (c *Config) GetTelemetryConfig() TelemetryConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.Telemetry
}

// NewConfigWatcher creates a new ConfigWatcher
func NewConfigWatcher(filePath string) *ConfigWatcher {
	return &ConfigWatcher{
		filePath: filePath,
		watchers: make([]func(*Config), 0),
	}
}

// Watch adds a callback function to be called when the config changes
func (cw *ConfigWatcher) Watch(callback func(*Config)) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	cw.watchers = append(cw.watchers, callback)
}

// Start starts the config watcher
func (cw *ConfigWatcher) Start() {
	go func() {
		for {
			cw.checkForUpdate()
			time.Sleep(1 * time.Second)
		}
	}()
}

// checkForUpdate checks if the config file has been updated
func (cw *ConfigWatcher) checkForUpdate() {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	fileInfo, err := os.Stat(cw.filePath)
	if err != nil {
		return
	}

	if fileInfo.ModTime().After(cw.lastMod) {
		cw.lastMod = fileInfo.ModTime()
		if err := Validate(cw.filePath); err != nil {
			logger := lg.GetInstance()
			logger.Error("update config error - type: %T, detail: %v", err, err)

			switch e := err.(type) {
			case *os.PathError:
				logger.Error("config file access error - operation[%s] path[%s]", e.Op, e.Path)
			default:
				logger.Error("config error - %v", err)
			}
			return
		}

		cfg := NewConfig()
		if err := cfg.LoadFromFile(cw.filePath); err != nil {
			return
		}

		for _, watcher := range cw.watchers {
			watcher(cfg)
		}
	}
}

// 中间结构用于解析列表
type rawConfig struct {
	ListenAddr  string            `yaml:"listen_addr" json:"listen_addr"`
	LogLevel    string            `yaml:"log_level" json:"log_level"`
	Telemetry   TelemetryConfig   `yaml:"telemetry" json:"telemetry"`
	Services    []*ServiceConfig  `yaml:"services" json:"services"` // 临时用 slice 解析
	HealthCheck HealthCheckConfig `yaml:"health_check" json:"health_check"`
}

// 服务配置结构需要添加 Name 字段
type ServiceConfig struct {
	Name         string         `yaml:"name" json:"name"`
	BalancerType string         `yaml:"balancer_type" json:"balancer_type"`
	Servers      []ServerConfig `yaml:"servers" json:"servers"`
}

// UnmarshalYAML 实现列表到 map 的转换
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw rawConfig
	if err := unmarshal(&raw); err != nil {
		return err
	}

	// 转换列表到 map
	services := make(map[string]*ServiceConfig)
	for _, svc := range raw.Services {
		if svc.Name == "" {
			return fmt.Errorf("service name is required")
		}
		if _, exists := services[svc.Name]; exists {
			return fmt.Errorf("duplicate service name: %s", svc.Name)
		}
		services[svc.Name] = svc
	}

	c.ListenAddr = raw.ListenAddr
	c.LogLevel = raw.LogLevel
	c.Telemetry = raw.Telemetry
	c.Services = services
	c.HealthCheck = raw.HealthCheck

	return nil
}

// UnmarshalJSON 实现列表到 map 的转换
func (c *Config) UnmarshalJSON(data []byte) error {
	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// 转换列表到 map
	services := make(map[string]*ServiceConfig)
	for _, svc := range raw.Services {
		if svc.Name == "" {
			return fmt.Errorf("service name is required")
		}
		if _, exists := services[svc.Name]; exists {
			return fmt.Errorf("duplicate service name: %s", svc.Name)
		}
		services[svc.Name] = svc
	}

	c.ListenAddr = raw.ListenAddr
	c.LogLevel = raw.LogLevel
	c.Telemetry = raw.Telemetry
	c.Services = services
	c.HealthCheck = raw.HealthCheck

	return nil
}
