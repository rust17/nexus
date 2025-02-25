package config

import (
	"errors"
	"fmt"
	"time"
)

// Validate Validate config file
func Validate(filePath string) error {
	c := NewConfig()

	if err := c.LoadFromFile(filePath); err != nil {
		return err
	}

	// Use validation functions instead of original logic
	if err := validateListenAddr(c.ListenAddr); err != nil {
		return err
	}
	if err := validateLogLevel(c.LogLevel); err != nil {
		return err
	}

	// Validate each service
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

	// Validate route config
	for _, route := range c.Routes {
		if err := validateRoute(route); err != nil {
			return fmt.Errorf("route %s: %w", route.Name, err)
		}
	}

	return validateHealthCheck(c.HealthCheck.Interval, c.HealthCheck.Timeout)
}

// validateListenAddr Validate listen address
func validateListenAddr(addr string) error {
	if addr == "" {
		return errors.New("listen address cannot be empty")
	}

	return nil
}

// validateBalancerType Validate balancer type
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

// validateLogLevel Validate log level
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

// validateServers Validate server list
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

// validateHealthCheck Validate health check config
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

// validateRoute Validate route config
func validateRoute(route *RouteConfig) error {
	if route.Name == "" {
		return errors.New("route name cannot be empty")
	}
	if route.Match.Path == "" && route.Match.Method == "" && route.Match.Host == "" && len(route.Match.Headers) == 0 {
		return fmt.Errorf("route %s: match condition cannot be empty", route.Name)
	}
	if route.Service == "" && len(route.Split) == 0 {
		return fmt.Errorf("route %s: must specify either service or split", route.Name)
	}
	if len(route.Split) > 0 {
		totalWeight := 0
		for _, split := range route.Split {
			if split.Service == "" {
				return fmt.Errorf("route %s: split service cannot be empty", route.Name)
			}
			if split.Weight <= 0 {
				return fmt.Errorf("route %s: split weight must be positive", route.Name)
			}
			totalWeight += split.Weight
		}
		if totalWeight != 100 {
			return fmt.Errorf("route %s: split weights must sum to 100", route.Name)
		}
	}

	return nil
}
