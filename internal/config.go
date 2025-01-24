package internal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 结构体包含所有配置项
type Config struct {
	mu sync.RWMutex

	// 服务器配置
	ListenAddr string `yaml:"listen_addr" json:"listen_addr"`

	// 负载均衡配置
	BalancerType string            `yaml:"balancer_type" json:"balancer_type"`
	Servers      []string          `yaml:"servers" json:"servers"`
	HealthCheck  HealthCheckConfig `yaml:"health_check" json:"health_check"`

	// 日志配置
	LogLevel string `yaml:"log_level" json:"log_level"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Interval time.Duration `yaml:"interval" json:"interval"`
	Timeout  time.Duration `yaml:"timeout" json:"timeout"`
}

// NewConfig 创建一个新的配置实例
func NewConfig() *Config {
	return &Config{
		ListenAddr:   ":8080",
		BalancerType: "round_robin",
		LogLevel:     "info",
		HealthCheck: HealthCheckConfig{
			Interval: 10 * time.Second,
			Timeout:  2 * time.Second,
		},
	}
}

// LoadFromFile 从文件加载配置
func (c *Config) LoadFromFile(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// 根据文件扩展名决定使用 YAML 还是 JSON
	switch filepath.Ext(path) {
	case ".yaml", ".yml":
		return yaml.Unmarshal(data, c)
	case ".json":
		return json.Unmarshal(data, c)
	default:
		return errors.New("unsupported config file format")
	}
}

// SaveToFile 保存配置到文件
func (c *Config) SaveToFile(path string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var data []byte
	var err error

	// 根据文件扩展名决定使用 YAML 还是 JSON
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

// GetListenAddr 获取监听地址
func (c *Config) GetListenAddr() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.ListenAddr
}

// GetBalancerType 获取负载均衡器类型
func (c *Config) GetBalancerType() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.BalancerType
}

// GetServers 获取服务器列表
func (c *Config) GetServers() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.Servers
}

// GetHealthCheckConfig 获取健康检查配置
func (c *Config) GetHealthCheckConfig() HealthCheckConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.HealthCheck
}

// GetLogLevel 获取日志级别
func (c *Config) GetLogLevel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.LogLevel
}
