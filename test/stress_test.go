package test

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cfg "nexus/internal/config"
	"nexus/internal/healthcheck"
	px "nexus/internal/proxy"
	"nexus/internal/route"
)

// Test configuration
type stressTestConfig struct {
	name              string        // Test name
	balancerType      string        // Type of load balancer
	concurrency       int           // Number of concurrent clients
	requestsPerClient int           // Requests per client
	backendCount      int           // Number of backend servers
	backendDelay      time.Duration // Backend processing delay
	timeout           time.Duration // Client timeout
}

// Test results
type stressTestResult struct {
	totalRequests   int           // Total requests
	successRequests int           // Successful requests
	failedRequests  int           // Failed requests
	totalDuration   time.Duration // Total test duration
	minLatency      time.Duration // Minimum latency
	maxLatency      time.Duration // Maximum latency
	avgLatency      time.Duration // Average latency
	p95Latency      time.Duration // 95th percentile latency
	p99Latency      time.Duration // 99th percentile latency
	requestsPerSec  float64       // Requests per second
	successPerSec   float64       // Successful requests per second
	successRate     float64       // Success rate
}

func TestStress(t *testing.T) {
	t.Parallel() // Mark test as parallelizable

	// Define test scenarios
	testCases := []stressTestConfig{
		// Round Robin - Different loads
		{"RoundRobin_LowLoad", "round_robin", 10, 10, 5, 10 * time.Millisecond, 1 * time.Second},
		{"RoundRobin_MediumLoad", "round_robin", 50, 20, 5, 10 * time.Millisecond, 1 * time.Second},
		{"RoundRobin_HighLoad", "round_robin", 100, 100, 5, 10 * time.Millisecond, 1 * time.Second},

		// Weighted Round Robin - Different loads
		{"WeightedRoundRobin_LowLoad", "weighted_round_robin", 10, 10, 5, 10 * time.Millisecond, 1 * time.Second},
		{"WeightedRoundRobin_MediumLoad", "weighted_round_robin", 50, 20, 5, 10 * time.Millisecond, 1 * time.Second},
		{"WeightedRoundRobin_HighLoad", "weighted_round_robin", 100, 100, 5, 10 * time.Millisecond, 1 * time.Second},

		// Least Connections - Different loads
		{"LeastConnections_LowLoad", "least_connections", 10, 10, 5, 10 * time.Millisecond, 1 * time.Second},
		{"LeastConnections_MediumLoad", "least_connections", 50, 20, 5, 10 * time.Millisecond, 1 * time.Second},
		{"LeastConnections_HighLoad", "least_connections", 100, 100, 5, 10 * time.Millisecond, 1 * time.Second},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Execute test
			result := runStressTest(t, tc)

			// Print results
			t.Logf("=== Statistics ===")
			t.Logf("Total requests: %d", result.totalRequests)
			t.Logf("Successful requests: %d", result.successRequests)
			t.Logf("Failed requests: %d", result.failedRequests)
			t.Logf("Success rate: %.2f%%", result.successRate*100)
			t.Logf("Total duration: %v", result.totalDuration)
			t.Logf("Requests per second (QPS): %.2f", result.requestsPerSec)
			t.Logf("Successful requests per second: %.2f", result.successPerSec)
			t.Logf("Minimum latency: %v", result.minLatency)
			t.Logf("Maximum latency: %v", result.maxLatency)
			t.Logf("Average latency: %v", result.avgLatency)
			t.Logf("P95 latency: %v", result.p95Latency)
			t.Logf("P99 latency: %v", result.p99Latency)
		})
	}
}

// runStressTest runs a stress test scenario and returns results
func runStressTest(t *testing.T, config stressTestConfig) stressTestResult {
	// Create test backend servers
	backends := createTestBackends(t, config.backendCount, config.backendDelay)

	// Initialize health checker (declared but not used yet)
	_ = initHealthChecker(t, backends)

	// Create route configuration
	routeConfigs := []*cfg.RouteConfig{
		{
			Name:    "test-route",
			Service: "test-service",
			Match: cfg.RouteMatch{
				Path: "/",
			},
		},
	}

	// Create service configuration
	svcConfig := &cfg.ServiceConfig{
		Name:         "test-service",
		BalancerType: config.balancerType,
		Servers:      convertToServerConfigs(backends),
	}

	// Create service mapping
	serviceConfigs := map[string]*cfg.ServiceConfig{
		"test-service": svcConfig,
	}

	// Initialize router
	router := route.NewRouter(routeConfigs, serviceConfigs)

	// Initialize reverse proxy
	proxy := px.NewProxy(router)

	// Start proxy server
	proxyServer := httptest.NewServer(proxy)
	t.Cleanup(func() { proxyServer.Close() })

	// Wait group and atomic counters
	var wg sync.WaitGroup
	var successCount int32
	var failedCount int32
	wg.Add(config.concurrency)

	// Slice to store latency data
	latencies := make([]time.Duration, 0, config.concurrency*config.requestsPerClient)
	var latenciesMutex sync.Mutex

	// Start time
	start := time.Now()

	// Launch concurrent clients
	for i := 0; i < config.concurrency; i++ {
		go func() {
			defer wg.Done()
			client := &http.Client{Timeout: config.timeout}

			for j := 0; j < config.requestsPerClient; j++ {
				reqStart := time.Now()
				err := makeRequest(t, client, proxyServer.URL)
				reqDuration := time.Since(reqStart)

				// Record latency
				latenciesMutex.Lock()
				latencies = append(latencies, reqDuration)
				latenciesMutex.Unlock()

				// Count success/failure
				if err != nil {
					atomic.AddInt32(&failedCount, 1)
				} else {
					atomic.AddInt32(&successCount, 1)
				}
			}
		}()
	}

	// Wait for all requests to complete
	wg.Wait()

	// Calculate total duration
	totalDuration := time.Since(start)
	totalRequests := config.concurrency * config.requestsPerClient

	// Calculate statistics
	result := calculateResults(
		totalRequests,
		int(successCount),
		int(failedCount),
		totalDuration,
		latencies,
	)

	return result
}

// createTestBackends creates test backend servers
func createTestBackends(t *testing.T, count int, delay time.Duration) []*httptest.Server {
	backends := make([]*httptest.Server, count)
	for i := range backends {
		i := i // Capture loop variable
		backends[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate processing time
			time.Sleep(delay)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Response from backend %d", i)
		}))
		t.Cleanup(func() { backends[i].Close() })
	}
	return backends
}

// initHealthChecker initializes and starts health checker
func initHealthChecker(t *testing.T, backends []*httptest.Server) *healthcheck.HealthChecker {
	healthChecker := healthcheck.NewHealthChecker(1*time.Second, 500*time.Millisecond)
	for _, backend := range backends {
		healthChecker.AddServer(backend.URL)
	}
	go healthChecker.Start()
	t.Cleanup(func() { healthChecker.Stop() })
	return healthChecker
}

// calculateResults calculates test result statistics
func calculateResults(totalRequests, successCount, failedCount int, totalDuration time.Duration, latencies []time.Duration) stressTestResult {
	// Sort latencies for percentile calculation
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	// Calculate min, max and average latency
	var minLatency time.Duration = math.MaxInt64
	var maxLatency time.Duration
	var totalLatency time.Duration

	if len(latencies) > 0 {
		minLatency = latencies[0]
		maxLatency = latencies[len(latencies)-1]

		for _, l := range latencies {
			totalLatency += l
		}
	}

	var avgLatency time.Duration
	if len(latencies) > 0 {
		avgLatency = time.Duration(int64(totalLatency) / int64(len(latencies)))
	}

	// Calculate percentile latencies
	p95Index := int(math.Ceil(float64(len(latencies))*0.95)) - 1
	p99Index := int(math.Ceil(float64(len(latencies))*0.99)) - 1

	var p95Latency, p99Latency time.Duration
	if len(latencies) > 0 {
		if p95Index >= 0 && p95Index < len(latencies) {
			p95Latency = latencies[p95Index]
		}
		if p99Index >= 0 && p99Index < len(latencies) {
			p99Latency = latencies[p99Index]
		}
	}

	// Calculate requests per second and success rate
	requestsPerSec := float64(totalRequests) / totalDuration.Seconds()
	successPerSec := float64(successCount) / totalDuration.Seconds()
	successRate := float64(successCount) / float64(totalRequests)

	return stressTestResult{
		totalRequests:   totalRequests,
		successRequests: successCount,
		failedRequests:  failedCount,
		totalDuration:   totalDuration,
		minLatency:      minLatency,
		maxLatency:      maxLatency,
		avgLatency:      avgLatency,
		p95Latency:      p95Latency,
		p99Latency:      p99Latency,
		requestsPerSec:  requestsPerSec,
		successPerSec:   successPerSec,
		successRate:     successRate,
	}
}

// makeRequest helper function for sending HTTP requests
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

	// Read and discard response body to ensure connection closure
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		return err
	}

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
	return fmt.Sprintf("HTTP error: %s (status code: %d)", http.StatusText(e.statusCode), e.statusCode)
}

// convertToServerConfigs converts httptest.Server slice to config.ServerConfig slice
func convertToServerConfigs(backends []*httptest.Server) []cfg.ServerConfig {
	configs := make([]cfg.ServerConfig, len(backends))
	for i, backend := range backends {
		configs[i] = cfg.ServerConfig{
			Address: backend.URL,
			Weight:  1,
		}
	}
	return configs
}
