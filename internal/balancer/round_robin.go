package balancer

import (
	"errors"
	"nexus/internal/config"
	"sync"
)

// RoundRobinBalancer implements round-robin load balancing algorithm
type RoundRobinBalancer struct {
	mu      sync.RWMutex
	servers []string
	index   int
}

// NewRoundRobinBalancer creates a new round-robin load balancer
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{
		servers: make([]string, 0),
		index:   0,
	}
}

// Next returns the next available server address
func (b *RoundRobinBalancer) Next() (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.servers) == 0 {
		return "", errors.New("no servers available")
	}

	server := b.servers[b.index]
	b.index = (b.index + 1) % len(b.servers)
	return server, nil
}

// Add adds a new server address
func (b *RoundRobinBalancer) Add(server string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.servers = append(b.servers, server)
}

// Remove removes a server address
func (b *RoundRobinBalancer) Remove(server string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, s := range b.servers {
		if s == server {
			b.servers = append(b.servers[:i], b.servers[i+1:]...)
			if b.index >= len(b.servers) {
				b.index = 0
			}
			break
		}
	}
}

// UpdateServers updates the list of servers
func (b *RoundRobinBalancer) UpdateServers(servers []config.ServerConfig) {
	b.mu.Lock()
	defer b.mu.Unlock()

	newServers := make([]string, 0, len(servers))
	for _, server := range servers {
		newServers = append(newServers, server.Address)
	}

	b.servers = newServers
	b.index = 0
}

func (b *RoundRobinBalancer) GetServers() []string {
	return b.servers
}
