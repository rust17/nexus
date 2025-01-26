package balancer

import (
	"errors"
	"math"
	"sync"
)

// LeastConnectionsServer represents a server with its connection count
type LeastConnectionsServer struct {
	Server    string
	ConnCount int
}

// LeastConnectionsBalancer implements least connections load balancing algorithm
type LeastConnectionsBalancer struct {
	mu      sync.RWMutex
	servers []LeastConnectionsServer
}

// NewLeastConnectionsBalancer creates a new least connections load balancer
func NewLeastConnectionsBalancer() *LeastConnectionsBalancer {
	return &LeastConnectionsBalancer{
		servers: make([]LeastConnectionsServer, 0),
	}
}

// Next returns the next available server address
func (b *LeastConnectionsBalancer) Next() (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.servers) == 0 {
		return "", errors.New("no servers available")
	}

	var selectedServer *LeastConnectionsServer
	minConnections := math.MaxInt32
	allEqual := true
	firstConnCount := b.servers[0].ConnCount

	for i := range b.servers {
		server := &b.servers[i]

		// Check if all servers have the same connection count
		if server.ConnCount != firstConnCount {
			allEqual = false
		}

		// Find the server with the least connections
		if server.ConnCount < minConnections {
			minConnections = server.ConnCount
			selectedServer = server
		}
	}

	// If all servers have the same connection count, return the first server
	if allEqual {
		selectedServer = &b.servers[0]
	}

	// Increment connection count for selected server
	selectedServer.ConnCount++
	return selectedServer.Server, nil
}

// Add adds a new server address
func (b *LeastConnectionsBalancer) Add(server string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.servers = append(b.servers, LeastConnectionsServer{
		Server:    server,
		ConnCount: 0,
	})
}

// AddWithConnCount adds a new server address with a specific connection count
func (b *LeastConnectionsBalancer) AddWithConnCount(server string, connCount int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.servers = append(b.servers, LeastConnectionsServer{
		Server:    server,
		ConnCount: connCount,
	})
}

// Remove removes a server address
func (b *LeastConnectionsBalancer) Remove(server string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, s := range b.servers {
		if s.Server == server {
			b.servers = append(b.servers[:i], b.servers[i+1:]...)
			break
		}
	}
}

// Done decrements the connection count for a server
func (b *LeastConnectionsBalancer) Done(server string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, s := range b.servers {
		if s.Server == server && s.ConnCount > 0 {
			b.servers[i].ConnCount--
			break
		}
	}
}
