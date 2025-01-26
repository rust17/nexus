package test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nexus/internal"
	lb "nexus/internal/balancer"
)

func BenchmarkProxy(b *testing.B) {
	// Create test backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// Define benchmark scenarios
	benchmarks := []struct {
		name         string
		balancerType string
	}{
		{"RoundRobin", "round_robin"},
		{"WeightedRoundRobin", "weighted_round_robin"},
		{"LeastConnections", "least_connections"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Initialize load balancer
			balancer := lb.NewBalancer(bm.balancerType)
			if bm.balancerType == "weighted_round_robin" {
				if wrr, ok := balancer.(*lb.WeightedRoundRobinBalancer); ok {
					wrr.AddWithWeight(backend.URL, 1)
				}
			} else {
				balancer.Add(backend.URL)
			}

			// Initialize reverse proxy
			proxy := internal.NewProxy(balancer)

			// Create test request
			req := httptest.NewRequest("GET", "/", nil)

			b.ReportAllocs() // Report memory allocations
			b.ResetTimer()

			b.Run("Sequential", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					w := httptest.NewRecorder()
					proxy.ServeHTTP(w, req)
				}
			})

			b.Run("Parallel", func(b *testing.B) {
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						w := httptest.NewRecorder()
						proxy.ServeHTTP(w, req)
					}
				})
			})
		})
	}
}

func BenchmarkProxyWithMultipleBackends(b *testing.B) {
	// Create multiple test backend servers
	backends := make([]*httptest.Server, 10)
	for i := range backends {
		backends[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer backends[i].Close()
	}

	// Define benchmark scenarios
	benchmarks := []struct {
		name         string
		balancerType string
	}{
		{"RoundRobin", "round_robin"},
		{"WeightedRoundRobin", "weighted_round_robin"},
		{"LeastConnections", "least_connections"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Initialize load balancer
			balancer := lb.NewBalancer(bm.balancerType)
			for _, backend := range backends {
				if bm.balancerType == "weighted_round_robin" {
					if wrr, ok := balancer.(*lb.WeightedRoundRobinBalancer); ok {
						wrr.AddWithWeight(backend.URL, 1)
					}
				} else {
					balancer.Add(backend.URL)
				}
			}

			// Initialize reverse proxy
			proxy := internal.NewProxy(balancer)

			// Create test request
			req := httptest.NewRequest("GET", "/", nil)

			b.ReportAllocs()
			b.ResetTimer()

			b.Run("Sequential", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					w := httptest.NewRecorder()
					proxy.ServeHTTP(w, req)
				}
			})

			b.Run("Parallel", func(b *testing.B) {
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						w := httptest.NewRecorder()
						proxy.ServeHTTP(w, req)
					}
				})
			})
		})
	}
}

func BenchmarkProxyWithHealthCheck(b *testing.B) {
	// Create test backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// Define benchmark scenarios
	benchmarks := []struct {
		name         string
		balancerType string
	}{
		{"RoundRobin", "round_robin"},
		{"WeightedRoundRobin", "weighted_round_robin"},
		{"LeastConnections", "least_connections"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Initialize load balancer
			balancer := lb.NewBalancer(bm.balancerType)
			if bm.balancerType == "weighted_round_robin" {
				if wrr, ok := balancer.(*lb.WeightedRoundRobinBalancer); ok {
					wrr.AddWithWeight(backend.URL, 1)
				}
			} else {
				balancer.Add(backend.URL)
			}

			// Initialize health checker
			healthChecker := internal.NewHealthChecker(10*time.Second, 1*time.Second)
			healthChecker.AddServer(backend.URL)
			go healthChecker.Start()
			defer healthChecker.Stop()

			// Initialize reverse proxy
			proxy := internal.NewProxy(balancer)

			// Create test request
			req := httptest.NewRequest("GET", "/", nil)

			b.ReportAllocs()
			b.ResetTimer()

			b.Run("Sequential", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					w := httptest.NewRecorder()
					proxy.ServeHTTP(w, req)
				}
			})

			b.Run("Parallel", func(b *testing.B) {
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						w := httptest.NewRecorder()
						proxy.ServeHTTP(w, req)
					}
				})
			})
		})
	}
}
