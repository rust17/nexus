package test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nexus/internal"
)

func TestIntegration(t *testing.T) {
	// 创建测试后端服务器
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Response from backend 1"))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Response from backend 2"))
	}))
	defer backend2.Close()

	// 创建配置
	cfg := internal.NewConfig()
	cfg.Servers = []string{backend1.URL, backend2.URL}
	cfg.HealthCheck.Interval = 100 * time.Millisecond
	cfg.HealthCheck.Timeout = 1 * time.Second

	// 初始化负载均衡器
	balancer := internal.NewRoundRobinBalancer()
	for _, server := range cfg.GetServers() {
		balancer.Add(server)
	}
	balancer.Next() // 确保从第一个服务器开始

	// 初始化健康检查
	healthChecker := internal.NewHealthChecker(cfg.GetHealthCheckConfig().Interval, cfg.GetHealthCheckConfig().Timeout)
	for _, server := range cfg.GetServers() {
		healthChecker.AddServer(server)
	}
	go healthChecker.Start()
	defer healthChecker.Stop()

	// 初始化反向代理
	proxy := internal.NewProxy(balancer)

	t.Run("单个请求应返回成功状态码", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		proxy.ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("期望状态码 %d, 实际得到 %d", http.StatusOK, resp.StatusCode)
		}
	})

	t.Run("负载均衡应正确轮询后端服务器", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)

		// 定义测试用例
		testCases := []struct {
			name     string
			expected string
		}{
			{"第一次请求", "Response from backend 1"},
			{"第二次请求", "Response from backend 2"},
			{"第三次请求", "Response from backend 1"},
			{"第四次请求", "Response from backend 2"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				proxy.ServeHTTP(w, req)

				got := w.Body.String()
				if got != tc.expected {
					t.Errorf("期望响应 %q, 实际得到 %q", tc.expected, got)
				}
			})
		}
	})
}
