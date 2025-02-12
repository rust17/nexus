package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"nexus/internal/balancer"
)

// Service 服务接口
type Service interface {
	NextServer(ctx context.Context) (string, error)
	Balancer() balancer.Balancer
}

// ServiceSelector 服务选择器接口
type ServiceSelector interface {
	Select(ctx context.Context, r *http.Request, services map[string]Service) (Service, error)
}

// 示例路由策略实现（可按需扩展）
type PathBasedSelector struct{}

func (s *PathBasedSelector) Select(ctx context.Context, r *http.Request, services map[string]Service) (Service, error) {
	// 根据请求路径选择服务，例如 /api/a => serviceA
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) > 2 {
		if service, exists := services[pathParts[2]]; exists {
			return service, nil
		}
	}
	return nil, fmt.Errorf("no matching service")
}
