package test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nexus/internal"
)

const (
	healthCheckInterval = 100 * time.Millisecond
	healthCheckTimeout  = 1 * time.Second
	pollInterval        = 500 * time.Millisecond
	pollCount           = 10
)

func TestHealthChecker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		serverHandler http.HandlerFunc
		expectHealthy bool
	}{
		{
			name: "HealthyServer",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectHealthy: true,
		},
		{
			name: "UnhealthyServer",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectHealthy: false,
		},
		{
			name: "TimeoutServer",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(2 * time.Second)
				w.WriteHeader(http.StatusOK)
			},
			expectHealthy: false,
		},
	}

	for _, tt := range tests {
		tt := tt // 避免闭包问题
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// 创建测试服务器
			ts := httptest.NewServer(tt.serverHandler)
			defer ts.Close()

			// 创建健康检查器
			checker := internal.NewHealthChecker(healthCheckInterval, healthCheckTimeout)
			checker.AddServer(ts.URL)
			go checker.Start()
			defer checker.Stop()

			// 使用轮询方式等待健康检查结果
			var healthy bool
			for i := 0; i < pollCount; i++ {
				if checker.IsHealthy(ts.URL) == tt.expectHealthy {
					healthy = true
					break
				}
				time.Sleep(pollInterval)
			}

			if !healthy {
				t.Errorf("Expected server to be healthy=%v, but got %v", tt.expectHealthy, !tt.expectHealthy)
			}
		})
	}
}

func TestHealthChecker_RemoveServer(t *testing.T) {
	t.Parallel()

	// 创建测试服务器
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// 创建健康检查器
	checker := internal.NewHealthChecker(healthCheckInterval, healthCheckTimeout)
	checker.AddServer(ts.URL)
	go checker.Start()
	defer checker.Stop()

	// 等待健康检查执行
	time.Sleep(200 * time.Millisecond)

	// 移除服务器
	checker.RemoveServer(ts.URL)
	if checker.IsHealthy(ts.URL) {
		t.Error("Removed server should not be considered healthy")
	}
}
