package internal

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// HealthChecker is responsible for health checking
type HealthChecker struct {
	mu       sync.RWMutex
	servers  map[string]bool
	interval time.Duration
	timeout  time.Duration
	stopChan chan struct{}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(interval, timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		servers:  make(map[string]bool),
		interval: interval,
		timeout:  timeout,
		stopChan: make(chan struct{}),
	}
}

// AddServer adds a server to be health checked
func (h *HealthChecker) AddServer(server string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.servers[server] = true
}

// RemoveServer removes a server from health checking
func (h *HealthChecker) RemoveServer(server string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.servers, server)
}

// IsHealthy checks if a server is healthy
func (h *HealthChecker) IsHealthy(server string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.servers[server]
}

// Start begins the health checking process
func (h *HealthChecker) Start() {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.checkAllServers()
		case <-h.stopChan:
			return
		}
	}
}

// Stop terminates the health checking process
func (h *HealthChecker) Stop() {
	close(h.stopChan)
}

// checkAllServers checks the health status of all servers
func (h *HealthChecker) checkAllServers() {
	h.mu.RLock()
	servers := make([]string, 0, len(h.servers))
	for server := range h.servers {
		servers = append(servers, server)
	}
	h.mu.RUnlock()

	var wg sync.WaitGroup
	for _, server := range servers {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			h.checkServer(s)
		}(server)
	}
	wg.Wait()
}

// checkServer checks the health status of a single server
func (h *HealthChecker) checkServer(server string) {
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", server+"/health", nil)
	if err != nil {
		h.UpdateServerStatus(server, false)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		h.UpdateServerStatus(server, false)
		return
	}

	h.UpdateServerStatus(server, true)
}

// UpdateServerStatus updates the server's health status
func (h *HealthChecker) UpdateServerStatus(server string, healthy bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.servers[server] = healthy
}
