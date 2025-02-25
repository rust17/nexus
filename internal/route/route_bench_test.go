package route

import (
	"net/http/httptest"
	"testing"

	"nexus/internal/config"
)

func BenchmarkRouter_Match(b *testing.B) {
	// Define test cases
	tests := []struct {
		name   string
		method string
		path   string
		host   string
		header map[string]string
	}{
		{
			name:   "SimplePathMatch",
			method: "GET",
			path:   "/api/v1",
		},
		{
			name:   "RegexPathMatch",
			method: "GET",
			path:   "/user/123/profile",
		},
		{
			name:   "ComplexMatch",
			method: "POST",
			path:   "/data",
			host:   "api.example.com",
			header: map[string]string{"Content-Type": "application/json"},
		},
		{
			name:   "NoMatch",
			method: "GET",
			path:   "/not/found",
		},
	}

	registerTests := []struct {
		name   string
		method string
		path   string
		host   string
		header map[string]string
	}{
		{
			name:   "FakeTest1",
			method: "GET",
			path:   "/api/v2/users",
		},
		{
			name:   "FakeTest2",
			method: "POST",
			path:   "/data/analytics",
			host:   "analytics.example.com",
		},
		{
			name:   "FakeTest3",
			method: "PUT",
			path:   "/user/{id}/settings",
			header: map[string]string{"Authorization": "Bearer token"},
		},
		{
			name:   "FakeTest4",
			method: "DELETE",
			path:   "/resource/{category}/{id}",
		},
		{
			name:   "FakeTest5",
			method: "PATCH",
			path:   "/config/{section}",
			host:   "admin.example.com",
			header: map[string]string{"X-Request-ID": "12345"},
		},
		{
			name:   "FakeTest6",
			method: "GET",
			path:   "/search",
			header: map[string]string{"Accept-Language": "zh-CN"},
		},
		{
			name:   "FakeTest7",
			method: "GET",
			path:   "/api/v3/{version}/products/{category}/{id}/details",
			host:   "products.example.com",
			header: map[string]string{
				"X-API-Version": "2023",
				"X-Tenant-ID":   "tenant-123",
			},
		},
		{
			name:   "FakeTest8",
			method: "POST",
			path:   "/webhooks/{service}/callback",
			header: map[string]string{
				"X-Signature":     "sha256=abc123",
				"X-Webhook-Event": "user.created",
			},
		},
		{
			name:   "FakeTest9",
			method: "PUT",
			path:   "/organizations/{org_id}/teams/{team_id}/members/{user_id}",
			host:   "api-internal.example.com",
			header: map[string]string{
				"Authorization": "Basic dXNlcjpwYXNz",
				"X-Role":        "admin",
			},
		},
		{
			name:   "FakeTest10",
			method: "GET",
			path:   "/metrics/{app}/{instance}/health",
			host:   "monitoring.example.com",
			header: map[string]string{
				"Accept":       "application/json",
				"X-Cluster-ID": "prod-east-1",
				"X-Debug":      "true",
			},
		},
	}
	registerTests = append(registerTests, tests...)

	routes := []*config.RouteConfig{}
	services := map[string]*config.ServiceConfig{}
	for _, test := range registerTests {
		routes = append(routes, &config.RouteConfig{
			Name:    test.name,
			Service: test.name,
			Match: config.RouteMatch{
				Method:  test.method,
				Path:    test.path,
				Headers: test.header,
				Host:    test.host,
			},
		})
		services[test.name] = &config.ServiceConfig{Name: test.name, BalancerType: "round_robin"}
	}
	router := NewRouter(routes, services)

	// Run benchmark test
	for _, tc := range tests {
		b.Run(tc.name, func(b *testing.B) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.host != "" {
				req.Host = tc.host
			}
			for k, v := range tc.header {
				req.Header.Set(k, v)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				router.Match(req)
			}
		})
	}
}
