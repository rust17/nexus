package balancer

import (
	"testing"

	"nexus/internal/config"
)

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
			balancer := NewLeastConnectionsBalancer()
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
			balancer := NewLeastConnectionsBalancer()
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
