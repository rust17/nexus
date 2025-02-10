package balancer

import (
	"context"
	"testing"

	"nexus/internal/config"
)

func TestWeightedRoundRobinBalancer(t *testing.T) {
	testCases := []struct {
		name          string
		servers       []WeightedServer
		expectedOrder []string
	}{
		{
			name: "Basic Weighted Round Robin",
			servers: []WeightedServer{
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
			balancer := NewWeightedRoundRobinBalancer()
			for _, server := range tc.servers {
				balancer.AddWithWeight(server.Server, server.Weight)
			}

			for i, expected := range tc.expectedOrder {
				server, err := balancer.Next(context.Background())
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
			balancer := NewWeightedRoundRobinBalancer()
			balancer.UpdateServers(tc.initialServers)

			// update servers
			balancer.UpdateServers(tc.updatedServers)

			// verify scheduling order
			for i, expected := range tc.expectedOrder {
				server, err := balancer.Next(context.Background())
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if server != expected {
					t.Errorf("Iteration %d: expected %s, got %s", i, expected, server)
				}
			}

			// verify index reset
			firstServer, err := balancer.Next(context.Background())
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if firstServer != tc.expectedOrder[0] {
				t.Errorf("Expected first server %s after reset, got %s", tc.expectedOrder[0], firstServer)
			}
		})
	}
}
