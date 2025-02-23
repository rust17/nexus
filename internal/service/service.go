package service

import (
	"context"
	lb "nexus/internal/balancer"
	"nexus/internal/config"
	"sync"
)

// Service 服务接口
type Service interface {
	Name() string
	NextServer(ctx context.Context) (string, error)
	Balancer() lb.Balancer
	Update(config *config.ServiceConfig) error
}

// 基础服务实现
type serviceImpl struct {
	mu       sync.RWMutex
	name     string
	balancer lb.Balancer
}

func NewService(config *config.ServiceConfig) Service {
	return &serviceImpl{
		name:     config.Name,
		balancer: newBalancer(config),
	}
}

func (s *serviceImpl) Name() string {
	return s.name
}

func newBalancer(config *config.ServiceConfig) lb.Balancer {
	balancer := lb.NewBalancer(config.BalancerType)

	for _, server := range config.Servers {
		if wrr, ok := balancer.(*lb.WeightedRoundRobinBalancer); ok {
			wrr.AddWithWeight(server.Address, server.Weight)
		} else {
			balancer.Add(server.Address)
		}
	}

	return balancer
}

func (s *serviceImpl) Balancer() lb.Balancer {
	return s.balancer
}

func (s *serviceImpl) NextServer(ctx context.Context) (string, error) {
	return s.balancer.Next(ctx)
}

func (s *serviceImpl) Update(config *config.ServiceConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if config.BalancerType != s.balancer.Type() {
		s.balancer = newBalancer(config)
	} else {
		s.balancer.UpdateServers(config.Servers)
	}

	s.name = config.Name

	return nil
}
