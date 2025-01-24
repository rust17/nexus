package internal

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// HealthChecker 结构体负责健康检查
type HealthChecker struct {
	mu       sync.RWMutex
	servers  map[string]bool
	interval time.Duration
	timeout  time.Duration
	stopChan chan struct{}
}

// NewHealthChecker 创建一个新的健康检查器
func NewHealthChecker(interval, timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		servers:  make(map[string]bool),
		interval: interval,
		timeout:  timeout,
		stopChan: make(chan struct{}),
	}
}

// AddServer 添加一个需要健康检查的服务器
func (h *HealthChecker) AddServer(server string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.servers[server] = true
}

// RemoveServer 移除一个服务器
func (h *HealthChecker) RemoveServer(server string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.servers, server)
}

// IsHealthy 检查服务器是否健康
func (h *HealthChecker) IsHealthy(server string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.servers[server]
}

// Start 开始健康检查
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

// Stop 停止健康检查
func (h *HealthChecker) Stop() {
	close(h.stopChan)
}

// checkAllServers 检查所有服务器的健康状态
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

// checkServer 检查单个服务器的健康状态
func (h *HealthChecker) checkServer(server string) {
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", server+"/health", nil)
	if err != nil {
		h.updateServerStatus(server, false)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		h.updateServerStatus(server, false)
		return
	}

	h.updateServerStatus(server, true)
}

// updateServerStatus 更新服务器状态
func (h *HealthChecker) updateServerStatus(server string, healthy bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.servers[server] = healthy
}
