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

func TestRouter_Update(t *testing.T) {
	// 初始配置
	initialRoutes := []*config.RouteConfig{
		{
			Name:    "initial_route",
			Service: "service_a",
			Match: config.RouteMatch{
				Method: "GET",
				Path:   "/initial",
			},
		},
	}

	initialServices := map[string]*config.ServiceConfig{
		"service_a": {Name: "service_a", BalancerType: "round_robin"},
	}

	rt := NewRouter(initialRoutes, initialServices)

	// 测试更新后的配置
	updatedRoutes := []*config.RouteConfig{
		{
			Name:    "new_route",
			Service: "service_b",
			Match: config.RouteMatch{
				Method: "POST",
				Path:   "/updated",
			},
		},
		{
			Name:    "updated_route",
			Service: "service_a",
			Match: config.RouteMatch{
				Method: "PUT",
				Path:   "/existing",
			},
		},
	}

	updatedServices := map[string]*config.ServiceConfig{
		"service_a": {Name: "service_a", BalancerType: "least_conn"}, // 更新现有服务配置
		"service_b": {Name: "service_b", BalancerType: "random"},     // 新增服务
	}

	// 执行更新操作
	if err := rt.Update(updatedRoutes, updatedServices); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// 验证服务更新
	t.Run("Service updates", func(t *testing.T) {
		// 转换为具体类型以访问私有字段
		r := rt.(*router)

		// 检查保留的服务是否更新
		if svc, ok := r.services["service_a"]; !ok {
			t.Error("Existing service should be preserved")
		} else if svc.Name() != "service_a" {
			t.Errorf("Expected service_a, got %s", svc.Name())
		}

		// 检查新增服务
		if _, ok := r.services["service_b"]; !ok {
			t.Error("New service should be added")
		}

		// 检查服务数量
		if len(r.services) != 2 {
			t.Errorf("Expected 2 services, got %d", len(r.services))
		}
	})

	// 验证路由更新
	t.Run("Route updates", func(t *testing.T) {
		testCases := []struct {
			method   string
			path     string
			expected string
		}{
			{"POST", "/updated", "service_b"}, // 新路由
			{"PUT", "/existing", "service_a"}, // 更新后的路由
			{"GET", "/initial", ""},           // 旧路由应该被移除
		}

		for _, tc := range testCases {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			svc := rt.Match(req)

			if tc.expected == "" {
				if svc != nil {
					t.Errorf("Expected no match for %s %s, got %s", tc.method, tc.path, svc.Name())
				}
				continue
			}

			if svc == nil || svc.Name() != tc.expected {
				t.Errorf("For %s %s: expected %s, got %v",
					tc.method, tc.path, tc.expected, svc)
			}
		}
	})

	// 测试部分更新
	t.Run("Partial update", func(t *testing.T) {
		partialRoutes := []*config.RouteConfig{
			{
				Name:    "partial_route",
				Service: "service_c",
				Match: config.RouteMatch{
					Method: "PATCH",
					Path:   "/partial",
				},
			},
		}

		partialServices := map[string]*config.ServiceConfig{
			"service_c": {Name: "service_c", BalancerType: "round_robin"},
		}

		if err := rt.Update(partialRoutes, partialServices); err != nil {
			t.Fatalf("Partial update failed: %v", err)
		}

		// 转换为具体类型以访问私有字段
		r := rt.(*router)
		if len(r.services) != 1 {
			t.Errorf("Expected 1 service after partial update, got %d", len(r.services))
		}

		// 验证新路由是否生效
		req := httptest.NewRequest("PATCH", "/partial", nil)
		if svc := rt.Match(req); svc == nil || svc.Name() != "service_c" {
			t.Errorf("Partial update failed to apply new route")
		}
	})
}
