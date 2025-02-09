package config

import "time"

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
func (c *Config) UpdateBalancerType(bType string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := validateBalancerType(bType); err != nil {
		return err
	}
	c.BalancerType = bType
	return nil
}

// UpdateServers 更新后端服务器列表
func (c *Config) UpdateServers(servers []ServerConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := validateServers(servers, c.BalancerType); err != nil {
		return err
	}

	c.Servers = append(c.Servers[:0], servers...)
	return nil
}

// UpdateHealthCheck 更新健康检查配置
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

// UpdateLogLevel 更新日志级别配置
func (c *Config) UpdateLogLevel(level string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := validateLogLevel(level); err != nil {
		return err
	}

	c.LogLevel = level
	return nil
}
