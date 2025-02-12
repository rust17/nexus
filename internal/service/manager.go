package service

import (
	"context"
	"net/http"
)

// 服务管理器实现
type ServiceManager struct {
	services map[string]Service
	selector ServiceSelector // 请求路由策略接口
}

func NewServiceManager() *ServiceManager {
	return &ServiceManager{
		services: make(map[string]Service),
	}
}

func (m *ServiceManager) GetService(name string) Service {
	return m.services[name]
}

func (m *ServiceManager) SelectService(ctx context.Context, r *http.Request) (Service, error) {
	return m.selector.Select(ctx, r, m.services)
}

// 注册服务实例
func (m *ServiceManager) RegisterService(name string, s Service) {
	m.services[name] = s
}
