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
	balancer := internal.NewRoundRobinBalancer()
	_, err := balancer.Next()
	if err == nil {
		t.Error("Expected error when no servers are available")
	}
}
