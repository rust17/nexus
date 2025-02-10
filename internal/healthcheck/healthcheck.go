package healthcheck

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	lg "nexus/internal/logger"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// HealthChecker is responsible for health checking
type HealthChecker struct {
	mu       sync.RWMutex
	servers  map[string]*serverInfo
	interval time.Duration
	timeout  time.Duration
	stopChan chan struct{}
}

type serverInfo struct {
	address string
	id      string
	healthy bool
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(interval, timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		servers:  make(map[string]*serverInfo),
		interval: interval,
		timeout:  timeout,
		stopChan: make(chan struct{}),
	}
}

// AddServer adds a server to be health checked
func (h *HealthChecker) AddServer(address string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.servers[address] = &serverInfo{
		address: address,
		healthy: true,
	}
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

	return h.servers[server] != nil && h.servers[server].healthy
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
	var wg sync.WaitGroup
	h.mu.RLock()
	servers := make([]*serverInfo, 0, len(h.servers))
	for _, s := range h.servers {
		servers = append(servers, s)
	}
	h.mu.RUnlock()

	for _, s := range servers {
		wg.Add(1)
		go func(s *serverInfo) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
			defer cancel()

			// 创建追踪span
			ctx, span := otel.Tracer("nexus.healthcheck").Start(ctx, "HealthCheck",
				trace.WithAttributes(
					attribute.String("service.address", s.address),
				))
			defer span.End()

			startTime := time.Now()
			err := h.httpCheck(ctx, s.address)
			duration := time.Since(startTime)

			// 记录检查结果
			span.SetAttributes(
				attribute.Bool("check.healthy", err == nil),
				attribute.Int64("check.duration_ms", duration.Milliseconds()),
			)

			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				lg.GetInstance().Error("[%s] 健康检查失败 - 耗时: %v 错误: %v",
					s.address, duration.Round(time.Millisecond), err)
			}

			h.UpdateServerStatus(s.address, err == nil)
		}(s)
	}
	wg.Wait()
}

func (h *HealthChecker) httpCheck(ctx context.Context, address string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", address+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("非正常状态码: %d", resp.StatusCode)
	}
	return nil
}

// UpdateServerStatus updates the server's health status
func (h *HealthChecker) UpdateServerStatus(server string, healthy bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if info, exists := h.servers[server]; exists {
		info.healthy = healthy
	}
}

// UpdateInterval updates the health checking interval
func (h *HealthChecker) UpdateInterval(newInterval time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.interval = newInterval
}

// UpdateTimeout updates the health checking timeout
func (h *HealthChecker) UpdateTimeout(newTimeout time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.timeout = newTimeout
}

func (h *HealthChecker) GetInterval() time.Duration {
	return h.interval
}

func (h *HealthChecker) GetTimeout() time.Duration {
	return h.timeout
}
