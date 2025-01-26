package balancer

import (
	"errors"
	"sync"
)

// WeightedServer represents a server with its weight
type WeightedServer struct {
	Server string
	Weight int
}

// WeightedRoundRobinBalancer implements weighted round-robin load balancing algorithm
type WeightedRoundRobinBalancer struct {
	mu            sync.RWMutex
	servers       []WeightedServer
	index         int
	current       int // current weight
	defaultWeight int // Default weight
}

// NewWeightedRoundRobinBalancer creates a new weighted round-robin load balancer
func NewWeightedRoundRobinBalancer() *WeightedRoundRobinBalancer {
	return &WeightedRoundRobinBalancer{
		servers:       make([]WeightedServer, 0),
		index:         0,
		current:       0,
		defaultWeight: 1, // Default weight is 1
	}
}

// Next returns the next available server address based on weight
func (b *WeightedRoundRobinBalancer) Next() (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.servers) == 0 {
		return "", errors.New("no servers available")
	}

	for {
		server := b.servers[b.index]
		if b.current < server.Weight {
			b.current++
			return server.Server, nil
		}

		b.current = 0
		b.index = (b.index + 1) % len(b.servers)
	}
}

// Add adds a new server address with default weight
func (b *WeightedRoundRobinBalancer) Add(server string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.servers = append(b.servers, WeightedServer{
		Server: server,
		Weight: b.defaultWeight,
	})
}

// AddWithWeight adds a new server address with specific weight
func (b *WeightedRoundRobinBalancer) AddWithWeight(server string, weight int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.servers = append(b.servers, WeightedServer{
		Server: server,
		Weight: weight,
	})
}

// Remove removes a server address
func (b *WeightedRoundRobinBalancer) Remove(server string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, s := range b.servers {
		if s.Server == server {
			b.servers = append(b.servers[:i], b.servers[i+1:]...)
			if b.index >= len(b.servers) {
				b.index = 0
			}
			break
		}
	}
}
