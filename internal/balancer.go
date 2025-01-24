package internal

import (
	"errors"
	"sync"
)

// Balancer 接口定义了负载均衡器的基本行为
type Balancer interface {
	Next() (string, error)
	Add(server string)
	Remove(server string)
}

// RoundRobinBalancer 实现轮询负载均衡算法
type RoundRobinBalancer struct {
	mu      sync.RWMutex
	servers []string
	index   int
}

// NewRoundRobinBalancer 创建一个新的轮询负载均衡器
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{
		servers: make([]string, 0),
		index:   0,
	}
}

// Next 返回下一个可用的服务器地址
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

// Add 添加一个新的服务器地址
func (b *RoundRobinBalancer) Add(server string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.servers = append(b.servers, server)
}

// Remove 移除一个服务器地址
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
