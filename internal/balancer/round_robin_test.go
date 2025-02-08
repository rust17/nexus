package balancer

import (
	"testing"

	"nexus/internal/config"
)

func TestRoundRobinBalancer(t *testing.T) {
	testCases := []struct {
		name          string
		servers       []string
		expectedOrder []string
		removeServer  string // Optional: server to be removed
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			balancer := NewRoundRobinBalancer()
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
				for i := 0; i < 3; i++ {
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
	balancer := NewRoundRobinBalancer()
	_, err := balancer.Next()
	if err == nil {
		t.Error("Expected error when no servers are available")
	}
}

func TestBalancer_UpdateRoundRobinServers(t *testing.T) {
	testCases := []struct {
		name           string
		initialServers []config.ServerConfig
		updatedServers []config.ServerConfig
		expectedCount  int
	}{
		{
			name: "Add new servers",
			initialServers: []config.ServerConfig{
				{Address: "http://server1:8080"},
			},
			updatedServers: []config.ServerConfig{
				{Address: "http://server1:8080"},
				{Address: "http://server2:8080"},
			},
			expectedCount: 2,
		},
		{
			name: "Remove servers",
			initialServers: []config.ServerConfig{
				{Address: "http://server1:8080"},
				{Address: "http://server2:8080"},
			},
			updatedServers: []config.ServerConfig{
				{Address: "http://server1:8080"},
			},
			expectedCount: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			balancer := NewRoundRobinBalancer()
			balancer.UpdateServers(tc.initialServers)
			balancer.UpdateServers(tc.updatedServers)

			// get servers
			servers := balancer.GetServers()
			if len(servers) != tc.expectedCount {
				t.Fatalf("Server count mismatch: expected %d, got %d", tc.expectedCount, len(servers))
			}

			// verify server addresses
			expectedSet := make(map[string]bool)
			for _, s := range tc.updatedServers {
				expectedSet[s.Address] = true
			}

			for _, server := range servers {
				if !expectedSet[server] {
					t.Errorf("Unexpected server %s in balancer", server)
				}
			}
		})
	}
}
