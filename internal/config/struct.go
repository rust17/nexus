package config

import (
	"sync"
	"time"
)

// Route config structure
type RouteConfig struct {
	Name    string        `yaml:"name" json:"name"`
	Match   RouteMatch    `yaml:"match" json:"match"`
	Service string        `yaml:"service" json:"service"`
	Split   []*RouteSplit `yaml:"split" json:"split"`
}

// Route match condition
type RouteMatch struct {
	Path    string            `yaml:"path" json:"path"`
	Headers map[string]string `yaml:"headers" json:"headers"`
	Method  string            `yaml:"method" json:"method"`
	Host    string            `yaml:"host" json:"host"`
}

// Traffic split configuration
type RouteSplit struct {
	Service string `yaml:"service" json:"service"`
	Weight  int    `yaml:"weight" json:"weight"`
}

// Intermediate temporary structure
type rawConfig struct {
	ListenAddr  string            `yaml:"listen_addr" json:"listen_addr"`
	LogLevel    string            `yaml:"log_level" json:"log_level"`
	Telemetry   TelemetryConfig   `yaml:"telemetry" json:"telemetry"`
	Services    []*ServiceConfig  `yaml:"services" json:"services"`
	Routes      []*RouteConfig    `yaml:"routes" json:"routes"`
	HealthCheck HealthCheckConfig `yaml:"health_check" json:"health_check"`
}

// Service config structure
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

	// Service list
	Services map[string]*ServiceConfig `yaml:"services" json:"services"`

	// Route config
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
