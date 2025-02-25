package config

import (
	"fmt"
	"time"
)

// UpdateListenAddr updates the listening address
func (c *Config) UpdateListenAddr(addr string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := validateListenAddr(addr); err != nil {
		return err
	}
	c.ListenAddr = addr
	return nil
}

// UpdateBalancerType updates the load balancer type
func (c *Config) UpdateBalancerType(serviceName string, bType string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := validateBalancerType(bType); err != nil {
		return err
	}

	sConfig, ok := c.Services[serviceName]
	if !ok {
		return fmt.Errorf("service %s not found", serviceName)
	}

	sConfig.BalancerType = bType
	return nil
}

// UpdateServers Update backend server list
func (c *Config) UpdateServers(serviceName string, servers []ServerConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	sConfig, ok := c.Services[serviceName]
	if !ok {
		return fmt.Errorf("service %s not found", serviceName)
	}

	if err := validateServers(servers, sConfig.BalancerType); err != nil {
		return err
	}

	sConfig.Servers = servers
	return nil
}

// UpdateHealthCheck Update health check config
func (c *Config) UpdateHealthCheck(interval, timeout time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := validateHealthCheck(interval, timeout); err != nil {
		return err
	}

	c.HealthCheck.Interval = interval
	c.HealthCheck.Timeout = timeout
	return nil
}

// UpdateLogLevel Update log level config
func (c *Config) UpdateLogLevel(level string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := validateLogLevel(level); err != nil {
		return err
	}

	c.LogLevel = level
	return nil
}

// UpdateRoutes Update route config
func (c *Config) UpdateRoutes(routes []*RouteConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, route := range routes {
		if err := validateRoute(route); err != nil {
			return err
		}
	}

	c.Routes = routes
	return nil
}
