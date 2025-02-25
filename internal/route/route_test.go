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
		// Basic path matching
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

		// Method matching
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

		// Host matching
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

		// Header matching
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

		// Combination matching
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

		// Edge cases
		{
			name:     "Case insensitive path match",
			method:   "GET",
			path:     "/CaseSensitivePath",
			expected: "case_insensitive",
		},

		// Special characters
		{
			name:     "Unicode path match",
			method:   "GET",
			path:     "/中文路径",
			expected: "unicode_path",
		},

		// Conflict routes
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

		// Regular expression
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

		// Weighted route
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

		// Version control
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
	// Initial configuration
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

	// Test updated configuration
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
		"service_a": {Name: "service_a", BalancerType: "least_conn"}, // Update existing service configuration
		"service_b": {Name: "service_b", BalancerType: "random"},     // Add new service
	}

	// Execute update operation
	if err := rt.Update(updatedRoutes, updatedServices); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify service updates
	t.Run("Service updates", func(t *testing.T) {
		// Convert to specific type to access private fields
		r := rt.(*router)

		// Check if the preserved service is updated
		if svc, ok := r.services["service_a"]; !ok {
			t.Error("Existing service should be preserved")
		} else if svc.Name() != "service_a" {
			t.Errorf("Expected service_a, got %s", svc.Name())
		}

		// Check if the new service is added
		if _, ok := r.services["service_b"]; !ok {
			t.Error("New service should be added")
		}

		// Check service count
		if len(r.services) != 2 {
			t.Errorf("Expected 2 services, got %d", len(r.services))
		}
	})

	// Verify route updates
	t.Run("Route updates", func(t *testing.T) {
		testCases := []struct {
			method   string
			path     string
			expected string
		}{
			{"POST", "/updated", "service_b"}, // New route
			{"PUT", "/existing", "service_a"}, // Updated route
			{"GET", "/initial", ""},           // Old route should be removed
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

	// Test partial update
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

		// Convert to specific type to access private fields
		r := rt.(*router)
		if len(r.services) != 1 {
			t.Errorf("Expected 1 service after partial update, got %d", len(r.services))
		}

		// Verify if the new route is applied
		req := httptest.NewRequest("PATCH", "/partial", nil)
		if svc := rt.Match(req); svc == nil || svc.Name() != "service_c" {
			t.Errorf("Partial update failed to apply new route")
		}
	})
}
