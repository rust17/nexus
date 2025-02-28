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

func BenchmarkSplitRouting(b *testing.B) {
	// Create different split scenarios
	splitScenarios := []struct {
		name        string
		splitCount  int    // Split service count
		equalWeight bool   // Whether the weight is equal
		path        string // Path pattern
	}{
		{
			name:        "TwoServices_EqualWeight",
			splitCount:  2,
			equalWeight: true,
			path:        "/api/two-equal",
		},
		{
			name:        "TwoServices_UnequalWeight",
			splitCount:  2,
			equalWeight: false,
			path:        "/api/two-unequal",
		},
		{
			name:        "FiveServices_EqualWeight",
			splitCount:  5,
			equalWeight: true,
			path:        "/api/five-equal",
		},
		{
			name:        "FiveServices_UnequalWeight",
			splitCount:  5,
			equalWeight: false,
			path:        "/api/five-unequal",
		},
		{
			name:        "TenServices_EqualWeight",
			splitCount:  10,
			equalWeight: true,
			path:        "/api/ten-equal",
		},
		{
			name:        "TenServices_UnequalWeight",
			splitCount:  10,
			equalWeight: false,
			path:        "/api/ten-unequal",
		},
		{
			name:        "TwentyServices_EqualWeight",
			splitCount:  20,
			equalWeight: true,
			path:        "/api/twenty-equal",
		},
		{
			name:        "TwentyServices_UnequalWeight",
			splitCount:  20,
			equalWeight: false,
			path:        "/api/twenty-unequal",
		},
		{
			name:        "FiftyServices_EqualWeight",
			splitCount:  50,
			equalWeight: true,
			path:        "/api/fifty-equal",
		},
		{
			name:        "FiftyServices_UnequalWeight",
			splitCount:  50,
			equalWeight: false,
			path:        "/api/fifty-unequal",
		},
		{
			name:        "HundredServices_EqualWeight",
			splitCount:  100,
			equalWeight: true,
			path:        "/api/hundred-equal",
		},
		{
			name:        "HundredServices_UnequalWeight",
			splitCount:  100,
			equalWeight: false,
			path:        "/api/hundred-unequal",
		},
	}

	routes := []*config.RouteConfig{}
	services := map[string]*config.ServiceConfig{}

	// Create route configurations for each split scenario
	for _, scenario := range splitScenarios {
		// Create service and split configurations
		splits := make([]*config.RouteSplit, scenario.splitCount)

		for i := 0; i < scenario.splitCount; i++ {
			serviceName := scenario.name + "_service_" + string(rune('A'+i))

			// Set weight for service
			weight := 100 / scenario.splitCount // Equal weight
			if !scenario.equalWeight {
				// Non-equal weight, use decreasing weight
				weight = 100 - i*2
				if weight <= 0 {
					weight = 1 // Ensure at least 1 weight
				}
			}

			// Add service configuration
			services[serviceName] = &config.ServiceConfig{
				Name:         serviceName,
				BalancerType: "round_robin",
			}

			// Add split configuration
			splits[i] = &config.RouteSplit{
				Service: serviceName,
				Weight:  weight,
			}
		}

		// Create route configuration
		routes = append(routes, &config.RouteConfig{
			Name: scenario.name,
			Match: config.RouteMatch{
				Path: scenario.path,
			},
			Split: splits,
		})
	}

	// Create router
	router := NewRouter(routes, services)

	// Run benchmark test
	for _, scenario := range splitScenarios {
		b.Run(scenario.name, func(b *testing.B) {
			// Create matching request
			req := httptest.NewRequest("GET", scenario.path, nil)

			// Reset timer
			b.ResetTimer()

			// Perform matching operation
			for i := 0; i < b.N; i++ {
				svc := router.Match(req)
				if svc == nil {
					b.Fatalf("Route matching failed: %s", scenario.path)
				}
			}
		})
	}
}

func BenchmarkSelectServiceBySplit(b *testing.B) {
	// Create a router instance for accessing its private method
	r := &router{}

	// Create different split scenarios for testing
	scenarios := []struct {
		name       string
		splitCount int
	}{
		{"TwoSplits", 2},
		{"FiveSplits", 5},
		{"TenSplits", 10},
		{"TwentySplits", 20},
		{"FiftySplits", 50},
		{"HundredSplits", 100},
		{"ThousandSplits", 1000},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			// Create a routeInfo with the specified number of split targets
			routeInfo := &routeInfo{
				split: make([]*config.RouteSplit, scenario.splitCount),
			}

			// Set service weight (use equal weight for simplicity)
			for i := 0; i < scenario.splitCount; i++ {
				routeInfo.split[i] = &config.RouteSplit{
					Service: "service_" + string(rune('A'+i%26)),
					Weight:  100 / scenario.splitCount,
				}
			}

			// Ensure at least 1 weight
			if routeInfo.split[0].Weight == 0 {
				for i := 0; i < scenario.splitCount; i++ {
					routeInfo.split[i].Weight = 1
				}
			}

			// Reset timer
			b.ResetTimer()

			// Perform service selection operation
			for i := 0; i < b.N; i++ {
				_ = r.selectServiceBySplit(routeInfo)
			}
		})
	}
}
