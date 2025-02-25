package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	lg "nexus/internal/logger"

	"gopkg.in/yaml.v3"
)

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

// GetRouteConfig gets the route configuration
func (c *Config) GetRouteConfig() []*RouteConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.Routes
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

// UnmarshalYAML 需要处理路由配置
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw rawConfig
	if err := unmarshal(&raw); err != nil {
		return err
	}

	// 转换服务列表到 map
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
	c.Routes = raw.Routes
	c.HealthCheck = raw.HealthCheck

	return nil
}

// UnmarshalJSON 需要处理路由配置
func (c *Config) UnmarshalJSON(data []byte) error {
	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// 转换服务列表到 map
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
	c.Routes = raw.Routes
	c.HealthCheck = raw.HealthCheck

	return nil
}
