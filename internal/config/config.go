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

	// Load balancer configuration
	BalancerType string            `yaml:"balancer_type" json:"balancer_type"`
	Servers      []ServerConfig    `yaml:"servers" json:"servers"`
	HealthCheck  HealthCheckConfig `yaml:"health_check" json:"health_check"`

	// Log configuration
	LogLevel string `yaml:"log_level" json:"log_level"`
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
		if err := yaml.Unmarshal(data, c); err != nil {
			return err
		}
	case ".json":
		if err := json.Unmarshal(data, c); err != nil {
			return err
		}
	default:
		return errors.New("unsupported config file format")
	}

	return c.validate()
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
func (c *Config) GetBalancerType() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.BalancerType
}

// GetServers gets the server list
func (c *Config) GetServers() []ServerConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.Servers
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
		cfg := NewConfig()
		if err := cfg.LoadFromFile(cw.filePath); err != nil {
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

		for _, watcher := range cw.watchers {
			watcher(cfg)
		}
	}
}

// validate validate config
func (c *Config) validate() error {
	// validate listen address
	if c.ListenAddr == "" {
		return errors.New("listen address cannot be empty")
	}

	// validate balancer type
	validBalancerTypes := map[string]bool{
		"round_robin":          true,
		"weighted_round_robin": true,
		"least_connections":    true,
	}
	if !validBalancerTypes[c.BalancerType] {
		return fmt.Errorf("invalid balancer type: %s", c.BalancerType)
	}

	// validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
	}
	if c.LogLevel != "" && !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log level: %s", c.LogLevel)
	}

	// validate server list
	if len(c.Servers) == 0 {
		return errors.New("at least one server must be configured")
	}
	for _, server := range c.Servers {
		if server.Address == "" {
			return errors.New("server address cannot be empty")
		}
		if c.BalancerType == "weighted_round_robin" && server.Weight <= 0 {
			return fmt.Errorf("invalid weight for server %s: %d", server.Address, server.Weight)
		}
	}

	// validate health check config
	if c.HealthCheck.Interval <= 0 {
		return errors.New("health check interval must be positive")
	}
	if c.HealthCheck.Timeout <= 0 {
		return errors.New("health check timeout must be positive")
	}
	if c.HealthCheck.Timeout >= c.HealthCheck.Interval {
		return errors.New("health check timeout must be less than interval")
	}

	return nil
}
