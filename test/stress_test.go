package test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	lb "nexus/internal/balancer"
	"nexus/internal/healthcheck"
	px "nexus/internal/proxy"
)

func TestStress(t *testing.T) {
	t.Parallel() // Mark test as parallel executable

	// Create multiple test backend servers
	backends := make([]*httptest.Server, 5)
	for i := range backends {
		backends[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond) // Simulate processing time
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(func() { backends[i].Close() }) // Use t.Cleanup instead of defer
	}

	// Define test scenarios
	testCases := []struct {
		name              string
		balancerType      string
		concurrency       int
		requestsPerClient int
	}{
		{"RoundRobin_LowLoad", "round_robin", 10, 10},
		{"RoundRobin_MediumLoad", "round_robin", 50, 20},
		{"RoundRobin_HighLoad", "round_robin", 100, 100},
		{"WeightedRoundRobin_LowLoad", "weighted_round_robin", 10, 10},
		{"WeightedRoundRobin_MediumLoad", "weighted_round_robin", 50, 20},
		{"WeightedRoundRobin_HighLoad", "weighted_round_robin", 100, 100},
		{"LeastConnections_LowLoad", "least_connections", 10, 10},
		{"LeastConnections_MediumLoad", "least_connections", 50, 20},
		{"LeastConnections_HighLoad", "least_connections", 100, 100},
	}

	for _, tc := range testCases {
		tc := tc // Capture loop variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Initialize load balancer
			balancer := lb.NewBalancer(tc.balancerType)
			for _, backend := range backends {
				if tc.balancerType == "weighted_round_robin" {
					if wrr, ok := balancer.(*lb.WeightedRoundRobinBalancer); ok {
						wrr.AddWithWeight(backend.URL, 1)
					}
				} else {
					balancer.Add(backend.URL)
				}
			}

			// Initialize health checker
			healthChecker := healthcheck.NewHealthChecker(1*time.Second, 500*time.Millisecond)
			for _, backend := range backends {
				healthChecker.AddServer(backend.URL)
			}
			go healthChecker.Start()
			t.Cleanup(func() { healthChecker.Stop() })

			// Initialize reverse proxy
			proxy := px.NewProxy(balancer)

			// Start proxy server
			proxyServer := httptest.NewServer(proxy)
			t.Cleanup(func() { proxyServer.Close() })

			// Wait group
			var wg sync.WaitGroup
			wg.Add(tc.concurrency)

			// Start time
			start := time.Now()

			// Launch concurrent clients
			for i := 0; i < tc.concurrency; i++ {
				go func() {
					defer wg.Done()
					client := &http.Client{Timeout: 1 * time.Second}
					for j := 0; j < tc.requestsPerClient; j++ {
						if err := makeRequest(t, client, proxyServer.URL); err != nil {
							t.Error("Request failed:", err)
						}
					}
				}()
			}

			// Wait for all requests to complete
			wg.Wait()

			// Calculate total duration
			duration := time.Since(start)
			totalRequests := tc.concurrency * tc.requestsPerClient
			t.Logf("Total requests: %d", totalRequests)
			t.Logf("Total time: %v", duration)
			t.Logf("Requests per second: %.2f", float64(totalRequests)/duration.Seconds())
		})
	}
}

// makeRequest is a helper function for sending HTTP requests
func makeRequest(t *testing.T, client *http.Client, url string) error {
	t.Helper() // Mark as helper function

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &httpError{statusCode: resp.StatusCode}
	}

	return nil
}

// httpError is a custom error type for handling HTTP errors
type httpError struct {
	statusCode int
}

func (e *httpError) Error() string {
	return http.StatusText(e.statusCode)
}
