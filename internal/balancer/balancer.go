package balancer

import (
	"nexus/internal/config"
)

// Balancer interface defines the basic behavior of a load balancer
type Balancer interface {
	Next() (string, error)
	Add(server string)
	Remove(server string)
	UpdateServers(servers []config.ServerConfig)
}

// NewBalancer creates a new load balancer based on the type
func NewBalancer(balancerType string) Balancer {
	switch balancerType {
	case "round_robin":
		return NewRoundRobinBalancer()
	case "least_connections":
		return NewLeastConnectionsBalancer()
	case "weighted_round_robin":
		return NewWeightedRoundRobinBalancer()
	default:
		return NewRoundRobinBalancer()
	}
}
