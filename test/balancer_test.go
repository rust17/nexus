package test

import (
	"testing"

	"nexus/internal"
)

func TestRoundRobinBalancer(t *testing.T) {
	testCases := []struct {
		name          string
		servers       []string
		expectedOrder []string
		removeServer  string // 可选：要移除的服务器
	}{
		{
			name: "Basic Round Robin",
			servers: []string{
				"http://server1:8080",
				"http://server2:8080",
				"http://server3:8080",
			},
			expectedOrder: []string{
				"http://server1:8080",
				"http://server2:8080",
				"http://server3:8080",
				"http://server1:8080",
			},
		},
		{
			name: "Round Robin after Remove",
			servers: []string{
				"http://server1:8080",
				"http://server2:8080",
				"http://server3:8080",
			},
			expectedOrder: []string{
				"http://server1:8080",
				"http://server3:8080",
				"http://server1:8080",
			},
			removeServer: "http://server2:8080",
		},
		// 可以添加更多测试用例，例如不同的服务器顺序，更多轮询次数等
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) { // 使用 t.Run 区分每个子测试用例
			balancer := internal.NewRoundRobinBalancer()
			for _, server := range tc.servers {
				balancer.Add(server)
			}

			if tc.removeServer != "" {
				balancer.Remove(tc.removeServer)
			}

			for i, expected := range tc.expectedOrder {
				server, err := balancer.Next()
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if server != expected {
					t.Errorf("Test case %s, iteration %d failed: expected %s, got %s", tc.name, i, expected, server)
				}
			}

			if tc.removeServer != "" {
				for i := 0; i < 3; i++ { // 多次调用 Next 确保移除的服务器不会再出现
					server, err := balancer.Next()
					if err != nil {
						t.Fatalf("Unexpected error after remove: %v", err)
					}
					if server == tc.removeServer {
						t.Errorf("Test case %s: Removed server '%s' should not be returned", tc.name, tc.removeServer)
					}
				}
			}
		})
	}
}

func TestEmptyBalancer(t *testing.T) {
	balancer := internal.NewRoundRobinBalancer()
	_, err := balancer.Next()
	if err == nil {
		t.Error("Expected error when no servers are available")
	}
}
