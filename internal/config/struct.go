package config

import (
	"sync"
	"time"
)

// 路由配置结构
type RouteConfig struct {
	Name    string        `yaml:"name" json:"name"`
	Match   RouteMatch    `yaml:"match" json:"match"`
	Service string        `yaml:"service" json:"service"`
	Split   []*RouteSplit `yaml:"split" json:"split"`
}

// 路由匹配条件
type RouteMatch struct {
	Path    string            `yaml:"path" json:"path"`
	Headers map[string]string `yaml:"headers" json:"headers"`
	Method  string            `yaml:"method" json:"method"`
	Host    string            `yaml:"host" json:"host"`
}

// 流量分割配置
type RouteSplit struct {
	Service string `yaml:"service" json:"service"`
	Weight  int    `yaml:"weight" json:"weight"`
}

// 中间临时结构
type rawConfig struct {
	ListenAddr  string            `yaml:"listen_addr" json:"listen_addr"`
	LogLevel    string            `yaml:"log_level" json:"log_level"`
	Telemetry   TelemetryConfig   `yaml:"telemetry" json:"telemetry"`
	Services    []*ServiceConfig  `yaml:"services" json:"services"`
	Routes      []*RouteConfig    `yaml:"routes" json:"routes"`
	HealthCheck HealthCheckConfig `yaml:"health_check" json:"health_check"`
}

// 服务配置结构
type ServiceConfig struct {
	Name         string         `yaml:"name" json:"name"`
	BalancerType string         `yaml:"balancer_type" json:"balancer_type"`
	Servers      []ServerConfig `yaml:"servers" json:"servers"`
}

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

	// 路由配置
	Routes []*RouteConfig `yaml:"routes" json:"routes"`

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
