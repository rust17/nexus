package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"nexus/internal/balancer"
	"nexus/internal/config"
	"nexus/internal/service"
)

type MockRouter struct {
	routes   []*config.RouteConfig
	services map[string]service.Service
}

func (m *MockRouter) Match(req *http.Request) service.Service {
	return m.services["mock"]
}

func (m *MockRouter) Update(routes []*config.RouteConfig, services map[string]*config.ServiceConfig) error {
	return nil
}

type MockService struct {
	backend *httptest.Server
}

func (m *MockService) Balancer() balancer.Balancer {
	balancer := balancer.NewBalancer("round_robin")
	if m.backend != nil {
		balancer.Add(m.backend.URL)
	}
	return balancer
}

func (m *MockService) NextServer(ctx context.Context) (string, error) {
	if m.backend != nil {
		return m.backend.URL, nil
	}
	return "", errors.New("no backend available")
}

func (m *MockService) Close() {
	if m.backend != nil {
		m.backend.Close()
	}
}

func (m *MockService) Name() string {
	return "mock_service"
}

func (m *MockService) Update(config *config.ServiceConfig) error {
	return nil
}
