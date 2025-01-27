package test

import (
	"testing"

	"nexus/internal/balancer"
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
		// More test cases can be added, such as different server orders, more polling rounds, etc.
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) { // Using t.Run to distinguish each sub-test case
			balancer := balancer.NewRoundRobinBalancer()
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
				for i := 0; i < 3; i++ { // Call Next multiple times to ensure removed server doesn't appear
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
	balancer := balancer.NewRoundRobinBalancer()
	_, err := balancer.Next()
	if err == nil {
		t.Error("Expected error when no servers are available")
	}
}

func TestLeastConnectionsBalancer(t *testing.T) {
	testCases := []struct {
		name          string
		servers       []map[string]int
		expectedOrder []string
		doneServer    string // Server to mark as done
		doneAfter     int    // Number of requests after which to call Done()
	}{
		{
			name: "Basic Least Connections",
			servers: []map[string]int{
				{"http://server1:8080": 1},
				{"http://server2:8080": 2},
			},
			expectedOrder: []string{
				"http://server1:8080",
				"http://server1:8080",
				"http://server2:8080",
			},
		},
		{
			name: "Least Connections with Done",
			servers: []map[string]int{
				{"http://server1:8080": 1},
				{"http://server2:8080": 2},
			},
			expectedOrder: []string{
				"http://server1:8080",
				"http://server1:8080",
				"http://server2:8080",
				"http://server1:8080",
			},
			doneServer: "http://server1:8080",
			doneAfter:  2, // Call Done() after 2 requests
		},
	}

	for _, tc := range testCases {
		tc := tc // Prevent closure issues
		t.Run(tc.name, func(t *testing.T) {
			balancer := balancer.NewLeastConnectionsBalancer()
			for _, server := range tc.servers {
				for serverAddr, connCount := range server {
					balancer.AddWithConnCount(serverAddr, connCount)
				}
			}

			for i, expected := range tc.expectedOrder {
				server, err := balancer.Next()
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if server != expected {
					t.Errorf("Test case %s, iteration %d failed: expected %s, got %s", tc.name, i, expected, server)
				}

				// Mark server as done if specified and after the specified number of requests
				if tc.doneServer != "" && server == tc.doneServer && i >= tc.doneAfter {
					balancer.Done(server)
				}
			}
		})
	}
}

func TestWeightedRoundRobinBalancer(t *testing.T) {
	testCases := []struct {
		name          string
		servers       []balancer.WeightedServer
		expectedOrder []string
	}{
		{
			name: "Basic Weighted Round Robin",
			servers: []balancer.WeightedServer{
				{Server: "http://server1:8080", Weight: 3},
				{Server: "http://server2:8080", Weight: 2},
			},
			expectedOrder: []string{
				"http://server1:8080",
				"http://server1:8080",
				"http://server1:8080",
				"http://server2:8080",
				"http://server2:8080",
				"http://server1:8080",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc // Prevent closure issues
		t.Run(tc.name, func(t *testing.T) {
			balancer := balancer.NewWeightedRoundRobinBalancer()
			for _, server := range tc.servers {
				balancer.AddWithWeight(server.Server, server.Weight)
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
		})
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
			balancer := balancer.NewRoundRobinBalancer()
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

func TestLeastConnections_UpdateServers(t *testing.T) {
	testCases := []struct {
		name           string
		initialServers []config.ServerConfig
		updatedServers []config.ServerConfig
		expectedCount  int
		expectedFirst  string
	}{
		{
			name: "Update with new servers",
			initialServers: []config.ServerConfig{
				{Address: "http://server1:8080"},
			},
			updatedServers: []config.ServerConfig{
				{Address: "http://server2:8080"},
				{Address: "http://server3:8080"},
			},
			expectedCount: 2,
			expectedFirst: "http://server2:8080",
		},
		{
			name: "Update with mixed servers",
			initialServers: []config.ServerConfig{
				{Address: "http://server1:8080"},
				{Address: "http://server2:8080"},
			},
			updatedServers: []config.ServerConfig{
				{Address: "http://server4:8080"},
				{Address: "http://server5:8080"},
			},
			expectedCount: 2,
			expectedFirst: "http://server4:8080",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			balancer := balancer.NewLeastConnectionsBalancer()
			balancer.UpdateServers(tc.initialServers)

			// update servers
			balancer.UpdateServers(tc.updatedServers)

			// verify server count
			servers := balancer.GetServers()
			if len(servers) != tc.expectedCount {
				t.Errorf("Expected %d servers, got %d", tc.expectedCount, len(servers))
			}

			// verify connection count reset
			firstServer, err := balancer.Next()
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if firstServer != tc.expectedFirst {
				t.Errorf("Expected first server %s, got %s", tc.expectedFirst, firstServer)
			}
		})
	}
}

func TestWeightedRoundRobin_UpdateServers(t *testing.T) {
	testCases := []struct {
		name           string
		initialServers []config.ServerConfig
		updatedServers []config.ServerConfig
		expectedOrder  []string
	}{
		{
			name: "Update weights",
			initialServers: []config.ServerConfig{
				{Address: "http://server1:8080", Weight: 2},
			},
			updatedServers: []config.ServerConfig{
				{Address: "http://server1:8080", Weight: 1},
				{Address: "http://server2:8080", Weight: 3},
			},
			expectedOrder: []string{
				"http://server1:8080",
				"http://server2:8080",
				"http://server2:8080",
				"http://server2:8080",
			},
		},
		{
			name: "Update with default weight",
			initialServers: []config.ServerConfig{
				{Address: "http://server1:8080", Weight: 2},
			},
			updatedServers: []config.ServerConfig{
				{Address: "http://server1:8080"}, // default weight
				{Address: "http://server2:8080", Weight: -1},
			},
			expectedOrder: []string{
				"http://server1:8080",
				"http://server2:8080",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			balancer := balancer.NewWeightedRoundRobinBalancer()
			balancer.UpdateServers(tc.initialServers)

			// update servers
			balancer.UpdateServers(tc.updatedServers)

			// verify scheduling order
			for i, expected := range tc.expectedOrder {
				server, err := balancer.Next()
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if server != expected {
					t.Errorf("Iteration %d: expected %s, got %s", i, expected, server)
				}
			}

			// verify index reset
			firstServer, err := balancer.Next()
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if firstServer != tc.expectedOrder[0] {
				t.Errorf("Expected first server %s after reset, got %s", tc.expectedOrder[0], firstServer)
			}
		})
	}
}
