package route

import (
	"net/http/httptest"
	"testing"

	"nexus/internal/config"
)

func TestRouter_Match(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		headers  map[string]string
		host     string
		expected string
	}{
		// 基础路径匹配
		{
			name:     "Exact path match",
			method:   "GET",
			path:     "/api/v1",
			expected: "exact_path",
		},
		{
			name:     "Wildcard path match",
			method:   "GET",
			path:     "/api/v2/any/sub/path",
			expected: "wildcard_path",
		},
		{
			name:     "Regex path match",
			method:   "GET",
			path:     "/user/123/profile",
			expected: "regex_path",
		},

		// 方法匹配
		{
			name:     "GET method match",
			method:   "GET",
			path:     "/api/method",
			expected: "get_method",
		},
		{
			name:     "POST method match",
			method:   "POST",
			path:     "/api/method",
			expected: "post_method",
		},

		// Host匹配
		{
			name:     "Exact host match",
			method:   "GET",
			path:     "/",
			host:     "api.example.com",
			expected: "host_match",
		},
		{
			name:     "Subdomain host match",
			method:   "GET",
			path:     "/",
			host:     "sub.example.com",
			expected: "subdomain_host",
		},

		// Header匹配
		{
			name:     "Content-Type header match",
			method:   "POST",
			path:     "/",
			headers:  map[string]string{"Content-Type": "application/json"},
			expected: "content_type",
		},
		{
			name:     "Authorization header match",
			method:   "GET",
			path:     "/",
			headers:  map[string]string{"Authorization": "Bearer token123"},
			expected: "auth_header",
		},

		// 组合匹配
		{
			name:     "Path and method match",
			method:   "PUT",
			path:     "/admin",
			expected: "path_method",
		},
		{
			name:     "Host and path match",
			method:   "GET",
			path:     "/dashboard",
			host:     "admin.example.com",
			expected: "host_path",
		},
		{
			name:     "Full match with all conditions",
			method:   "POST",
			path:     "/v3/data",
			host:     "data-center-01",
			headers:  map[string]string{"X-API-Key": "secret123", "Content-Type": "text/xml"},
			expected: "full_match",
		},

		// 边界情况
		{
			name:     "Case insensitive path match",
			method:   "GET",
			path:     "/CaseSensitivePath",
			expected: "case_insensitive",
		},

		// 特殊字符
		{
			name:     "Unicode path match",
			method:   "GET",
			path:     "/中文路径",
			expected: "unicode_path",
		},

		// 冲突路由
		{
			name:     "Conflict route 1 - basic path",
			method:   "GET",
			path:     "/conflict",
			expected: "conflict_1",
		},
		{
			name:     "Conflict route 2 - with DELETE method",
			method:   "DELETE",
			path:     "/conflict",
			expected: "conflict_2",
		},

		// 正则表达式
		{
			name:     "UUID path match",
			method:   "GET",
			path:     "/user/123e4567-e89b-12d3-a456-426614174000",
			expected: "uuid_path",
		},
		{
			name:     "Version path match",
			method:   "GET",
			path:     "/v2/any/path",
			expected: "version_path",
		},

		// 权重路由
		{
			name:     "Weighted route A",
			method:   "GET",
			path:     "/weighted",
			headers:  map[string]string{"X-Group": "A"},
			expected: "weighted_a",
		},
		{
			name:     "Weighted route B",
			method:   "GET",
			path:     "/weighted",
			headers:  map[string]string{"X-Group": "B"},
			expected: "weighted_b",
		},

		// 版本控制
		{
			name:     "Version header match",
			method:   "GET",
			path:     "/",
			headers:  map[string]string{"X-API-Version": "2023-07"},
			expected: "version_header",
		},
		{
			name:     "Accept header match",
			method:   "GET",
			path:     "/",
			headers:  map[string]string{"Accept": "application/vnd.v2+json"},
			expected: "accept_header",
		},
	}

	routes := []*config.RouteConfig{}
	services := map[string]*config.ServiceConfig{}
	for _, test := range tests {
		routes = append(routes, &config.RouteConfig{
			Name:    test.expected,
			Service: test.expected,
			Match: config.RouteMatch{
				Method:  test.method,
				Path:    test.path,
				Headers: test.headers,
				Host:    test.host,
			},
		})
		services[test.expected] = &config.ServiceConfig{Name: test.expected, BalancerType: "round_robin"}
	}
	router := NewRouter(routes, services)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.host != "" {
				req.Host = tt.host
			}
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			svc := router.Match(req)

			if tt.expected == "" {
				if svc != nil {
					t.Errorf("Expected no service match, got %v", svc)
				}
			} else {
				if svc == nil {
					t.Errorf("Expected service %s, got %v", tt.expected, svc)
				}
				if svc != nil && svc.Name() != tt.expected {
					t.Errorf("Expected service %s, got %v", tt.expected, svc.Name())
				}
			}
		})
	}
}
