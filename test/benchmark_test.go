package test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nexus/internal"
)

func BenchmarkProxy(b *testing.B) {
	// 创建测试后端服务器
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// 初始化负载均衡器
	balancer := internal.NewRoundRobinBalancer()
	balancer.Add(backend.URL)

	// 初始化反向代理
	proxy := internal.NewProxy(balancer)

	// 创建测试请求
	req := httptest.NewRequest("GET", "/", nil)

	b.ReportAllocs() // 报告内存分配
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
}

func BenchmarkProxyWithMultipleBackends(b *testing.B) {
	// 创建多个测试后端服务器
	backends := make([]*httptest.Server, 10)
	for i := range backends {
		backends[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer backends[i].Close()
	}

	// 初始化负载均衡器
	balancer := internal.NewRoundRobinBalancer()
	for _, backend := range backends {
		balancer.Add(backend.URL)
	}

	// 初始化反向代理
	proxy := internal.NewProxy(balancer)

	// 创建测试请求
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
}

func BenchmarkProxyWithHealthCheck(b *testing.B) {
	// 创建测试后端服务器
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// 初始化负载均衡器
	balancer := internal.NewRoundRobinBalancer()
	balancer.Add(backend.URL)

	// 初始化健康检查
	healthChecker := internal.NewHealthChecker(10*time.Second, 1*time.Second)
	healthChecker.AddServer(backend.URL)
	go healthChecker.Start()
	defer healthChecker.Stop()

	// 初始化反向代理
	proxy := internal.NewProxy(balancer)

	// 创建测试请求
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
}
