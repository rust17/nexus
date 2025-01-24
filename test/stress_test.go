package test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"nexus/internal"
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

	// Initialize load balancer
	balancer := internal.NewBalancer("round_robin")
	for _, backend := range backends {
		balancer.Add(backend.URL)
	}

	// Initialize health checker
	healthChecker := internal.NewHealthChecker(1*time.Second, 500*time.Millisecond)
	for _, backend := range backends {
		healthChecker.AddServer(backend.URL)
	}
	go healthChecker.Start()
	t.Cleanup(func() { healthChecker.Stop() })

	// Initialize reverse proxy
	proxy := internal.NewProxy(balancer)

	// Start proxy server
	proxyServer := httptest.NewServer(proxy)
	t.Cleanup(func() { proxyServer.Close() })

	// Define test scenarios
	testCases := []struct {
		name              string
		concurrency       int
		requestsPerClient int
	}{
		{"LowLoad", 10, 10},
		{"MediumLoad", 50, 20},
		{"HighLoad", 100, 100},
	}

	for _, tc := range testCases {
		tc := tc // Capture loop variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

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
