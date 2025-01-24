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
	t.Parallel() // 标记测试可以并行执行

	// 创建多个测试后端服务器
	backends := make([]*httptest.Server, 5)
	for i := range backends {
		backends[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond) // 模拟处理时间
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(func() { backends[i].Close() }) // 使用 t.Cleanup 替代 defer
	}

	// 初始化负载均衡器
	balancer := internal.NewRoundRobinBalancer()
	for _, backend := range backends {
		balancer.Add(backend.URL)
	}

	// 初始化健康检查
	healthChecker := internal.NewHealthChecker(1*time.Second, 500*time.Millisecond)
	for _, backend := range backends {
		healthChecker.AddServer(backend.URL)
	}
	go healthChecker.Start()
	t.Cleanup(func() { healthChecker.Stop() })

	// 初始化反向代理
	proxy := internal.NewProxy(balancer)

	// 启动代理服务器
	proxyServer := httptest.NewServer(proxy)
	t.Cleanup(func() { proxyServer.Close() })

	// 定义测试场景
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
		tc := tc // 捕获循环变量
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// 等待组
			var wg sync.WaitGroup
			wg.Add(tc.concurrency)

			// 启动时间
			start := time.Now()

			// 启动并发客户端
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

			// 等待所有请求完成
			wg.Wait()

			// 计算总耗时
			duration := time.Since(start)
			totalRequests := tc.concurrency * tc.requestsPerClient
			t.Logf("Total requests: %d", totalRequests)
			t.Logf("Total time: %v", duration)
			t.Logf("Requests per second: %.2f", float64(totalRequests)/duration.Seconds())
		})
	}
}

// makeRequest 是一个辅助函数，用于发送 HTTP 请求
func makeRequest(t *testing.T, client *http.Client, url string) error {
	t.Helper() // 标记为辅助函数

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

// httpError 是一个自定义错误类型，用于处理 HTTP 错误
type httpError struct {
	statusCode int
}

func (e *httpError) Error() string {
	return http.StatusText(e.statusCode)
}
