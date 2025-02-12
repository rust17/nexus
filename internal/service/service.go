package service

import (
	"context"
	"nexus/internal/balancer"
	"nexus/internal/healthcheck"
)

// 基础服务实现
type serviceImpl struct {
	name        string
	balancer    balancer.Balancer
	healthCheck *healthcheck.HealthChecker
	retryCount  int
}

func NewService(name string, balancer balancer.Balancer, checker *healthcheck.HealthChecker) Service {
	return &serviceImpl{
		name:        name,
		balancer:    balancer,
		healthCheck: checker,
		retryCount:  3, // 默认重试次数
	}
}

func (s *serviceImpl) Balancer() balancer.Balancer {
	return s.balancer
}

func (s *serviceImpl) NextServer(ctx context.Context) (string, error) {
	return s.balancer.Next(ctx)
}

func (s *serviceImpl) GetHealthChecker() *healthcheck.HealthChecker {
	return s.healthCheck
}
