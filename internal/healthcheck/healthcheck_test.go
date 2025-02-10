package healthcheck

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
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
		tt := tt // Prevent closure issues
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test server
			ts := httptest.NewServer(tt.serverHandler)
			defer ts.Close()

			// Create health checker
			checker := NewHealthChecker(healthCheckInterval, healthCheckTimeout)
			checker.AddServer(ts.URL)
			go checker.Start()
			defer checker.Stop()

			// Wait for health check results using polling
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

	// Create test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Create health checker
	checker := NewHealthChecker(healthCheckInterval, healthCheckTimeout)
	checker.AddServer(ts.URL)
	go checker.Start()
	defer checker.Stop()

	// Wait for health check execution
	time.Sleep(200 * time.Millisecond)

	// Remove server
	checker.RemoveServer(ts.URL)
	time.Sleep(100 * time.Millisecond) // 等待可能进行中的健康检查完成

	// 检查服务器是否已移除且不可访问
	checker.mu.RLock()
	defer checker.mu.RUnlock()
	if _, exists := checker.servers[ts.URL]; exists {
		t.Error("Server should be removed from the list")
	}
}

func TestHealthChecker_UpdateConfig(t *testing.T) {
	healthChecker := NewHealthChecker(10*time.Second, 1*time.Second)
	healthChecker.AddServer("http://server1:8080")

	// Update interval and timeout
	healthChecker.UpdateInterval(5 * time.Second)
	healthChecker.UpdateTimeout(500 * time.Millisecond)

	if healthChecker.GetInterval() != 5*time.Second {
		t.Errorf("Expected interval 5s, got %v", healthChecker.GetInterval())
	}

	if healthChecker.GetTimeout() != 500*time.Millisecond {
		t.Errorf("Expected timeout 500ms, got %v", healthChecker.GetTimeout())
	}
}

func TestHealthCheckTracing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		handler        http.HandlerFunc
		wantAttributes map[attribute.Key]attribute.Value
	}{
		{
			name: "SuccessfulCheck",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantAttributes: map[attribute.Key]attribute.Value{
				attribute.Key("service.address"):   attribute.StringValue(""),
				attribute.Key("check.healthy"):     attribute.BoolValue(true),
				attribute.Key("check.duration_ms"): attribute.Int64Value(0),
			},
		},
		{
			name: "FailedCheck",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantAttributes: map[attribute.Key]attribute.Value{
				attribute.Key("service.address"):   attribute.StringValue(""),
				attribute.Key("check.healthy"):     attribute.BoolValue(false),
				attribute.Key("check.duration_ms"): attribute.Int64Value(0),
			},
		},
	}

	for _, tt := range tests {
		tt := tt // Prevent closure issues
		t.Run(tt.name, func(t *testing.T) {
			// 创建span导出器
			exporter := tracetest.NewInMemoryExporter()
			tp := trace.NewTracerProvider(
				trace.WithSyncer(exporter),
			)

			// 替换全局的TracerProvider
			oldTP := otel.GetTracerProvider()
			defer otel.SetTracerProvider(oldTP)
			otel.SetTracerProvider(tp)

			// 创建测试服务器
			ts := httptest.NewServer(tt.handler)
			defer ts.Close()

			// 更新测试用例中的地址属性
			tt.wantAttributes[attribute.Key("service.address")] = attribute.StringValue(ts.URL)

			// 创建健康检查器
			checker := NewHealthChecker(10*time.Millisecond, 1*time.Second)
			checker.AddServer(ts.URL)
			go checker.Start()
			defer checker.Stop()

			// 等待健康检查完成
			time.Sleep(50 * time.Millisecond)

			// 验证追踪数据
			spans := exporter.GetSpans()
			if len(spans) == 0 {
				t.Fatal("No spans recorded")
			}

			span := spans[0]
			if span.Name != "HealthCheck" {
				t.Errorf("Span name got %q, want %q", span.Name, "HealthCheck")
			}

			gotAttrs := make(map[attribute.Key]attribute.Value)
			for _, attr := range span.Attributes {
				gotAttrs[attr.Key] = attr.Value
			}

			for k, want := range tt.wantAttributes {
				got, exists := gotAttrs[k]
				if !exists {
					t.Errorf("Missing attribute %q", k)
					continue
				}
				if k == "check.duration_ms" {
					if got.AsInt64() < 0 {
						t.Errorf("Duration should be >= 0, got %d", got.AsInt64())
					}
					continue
				}
				if got != want {
					t.Errorf("Attribute %q got %v, want %v", k, got, want)
				}
			}
		})
	}
}
