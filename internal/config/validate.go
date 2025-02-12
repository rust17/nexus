package config

import (
	"errors"
	"fmt"
	"time"
)

// Validate 校验配置文件
func Validate(filePath string) error {
	c := NewConfig()

	if err := c.LoadFromFile(filePath); err != nil {
		return err
	}

	// 使用校验函数替换原有逻辑
	if err := validateListenAddr(c.ListenAddr); err != nil {
		return err
	}
	if err := validateLogLevel(c.LogLevel); err != nil {
		return err
	}

	// 校验每个服务
	for _, svc := range c.Services {
		if svc.Name == "" {
			return errors.New("service name cannot be empty")
		}
		if err := validateBalancerType(svc.BalancerType); err != nil {
			return fmt.Errorf("service %s: %w", svc.Name, err)
		}
		if err := validateServers(svc.Servers, svc.BalancerType); err != nil {
			return fmt.Errorf("service %s: %w", svc.Name, err)
		}
	}

	return validateHealthCheck(c.HealthCheck.Interval, c.HealthCheck.Timeout)
}

// validateListenAddr 校验监听地址
func validateListenAddr(addr string) error {
	if addr == "" {
		return errors.New("listen address cannot be empty")
	}

	return nil
}

// validateBalancerType 校验负载均衡类型
func validateBalancerType(bType string) error {
	validTypes := map[string]bool{
		"round_robin":          true,
		"weighted_round_robin": true,
		"least_connections":    true,
	}
	if !validTypes[bType] {
		return fmt.Errorf("invalid balancer type: %s", bType)
	}

	return nil
}

// validateLogLevel 校验日志级别
func validateLogLevel(level string) error {
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
		"":      true,
	}
	if !validLevels[level] {
		return fmt.Errorf("invalid log level: %s", level)
	}

	return nil
}

// validateServers 校验服务器列表
func validateServers(servers []ServerConfig, balancerType string) error {
	if len(servers) == 0 {
		return errors.New("server list cannot be empty")
	}

	for _, server := range servers {
		if server.Address == "" {
			return errors.New("server address cannot be empty")
		}
		if balancerType == "weighted_round_robin" && server.Weight <= 0 {
			return fmt.Errorf("invalid weight for server %s: %d", server.Address, server.Weight)
		}
	}

	return nil
}

// validateHealthCheck 校验健康检查配置
func validateHealthCheck(interval, timeout time.Duration) error {
	if interval <= 0 {
		return errors.New("health check interval must be positive")
	}
	if timeout <= 0 {
		return errors.New("health check timeout must be positive")
	}
	if timeout >= interval {
		return errors.New("health check timeout must be less than interval")
	}

	return nil
}
