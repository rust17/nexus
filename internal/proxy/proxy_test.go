package proxy

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"nexus/internal/balancer"
	lb "nexus/internal/balancer"
	"nexus/internal/config"
	"nexus/internal/service"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

const (
	testResponseBody = "Hello from backend"
)

type MockRouter struct {
	routes   []*config.RouteConfig
	services map[string]service.Service
}

func (m *MockRouter) Match(req *http.Request) service.Service {
	return m.services["mock"]
}

type MockService struct {
	backend *httptest.Server
}

func (m *MockService) Balancer() balancer.Balancer {
	balancer := lb.NewBalancer("round_robin")
	if m.backend != nil {
		balancer.Add(m.backend.URL)
	}
	return balancer
}

func (m *MockService) NextServer(ctx context.Context) (string, error) {
	if m.backend != nil {
		return m.backend.URL, nil
	}
	return "", errors.New("no backend available")
}

func (m *MockService) Close() {
	if m.backend != nil {
		m.backend.Close()
	}
}

func (m *MockService) Name() string {
	return "mock_service"
}

func TestProxy_RequestFlow(t *testing.T) {
	tests := []struct {
		name         string
		setup        *MockService
		expectStatus int
		expectBody   string
	}{
		{
			name: "HealthyBackend",
			setup: &MockService{
				backend: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(testResponseBody))
				})),
			},
			expectStatus: http.StatusOK,
			expectBody:   testResponseBody,
		},
		{
			name:         "NoBackendAvailable",
			setup:        &MockService{},
			expectStatus: http.StatusServiceUnavailable,
			expectBody:   "Service unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := tt.setup
			defer mockSvc.Close()

			proxy := NewProxy(&MockRouter{
				services: map[string]service.Service{
					"mock": mockSvc,
				},
			})

			r := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			proxy.ServeHTTP(w, r)

			resp := w.Result()
			if resp.StatusCode != tt.expectStatus {
				t.Errorf("Expected status %d, got %d", tt.expectStatus, resp.StatusCode)
			}
			if !bytes.Contains(w.Body.Bytes(), []byte(tt.expectBody)) {
				t.Errorf("Expected body %q, got %q", tt.expectBody, w.Body.String())
			}
		})
	}
}

func TestProxy_Concurrency(t *testing.T) {
	mockSvc := &MockService{
		backend: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.Write([]byte(testResponseBody))
		})),
	}
	defer mockSvc.Close()

	proxy := NewProxy(&MockRouter{
		services: map[string]service.Service{
			"mock": mockSvc,
		},
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			proxy.ServeHTTP(w, r)
		}()
	}
	wg.Wait()
}

func TestProxy_CustomTransport(t *testing.T) {
	mockSvc := &MockService{
		backend: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(testResponseBody))
		})),
	}
	defer mockSvc.Close()

	proxy := NewProxy(&MockRouter{
		services: map[string]service.Service{
			"mock": mockSvc,
		},
	})

	customTransport := &http.Transport{
		ResponseHeaderTimeout: 1 * time.Second,
	}
	proxy.SetTransport(customTransport)

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, r)

	if proxy.transport != customTransport {
		t.Error("Custom transport not set properly")
	}
}

func TestProxy_ErrorHandler(t *testing.T) {
	tests := []struct {
		name          string
		setErrHandler func(*Proxy)
		expectStatus  int
	}{
		{
			name:          "DefaultHandler",
			setErrHandler: func(p *Proxy) {},
			expectStatus:  http.StatusServiceUnavailable,
		},
		{
			name: "CustomHandler",
			setErrHandler: func(p *Proxy) {
				p.SetErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
					w.WriteHeader(http.StatusBadGateway)
					w.Write([]byte("custom error"))
				})
			},
			expectStatus: http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := NewProxy(&MockRouter{
				services: map[string]service.Service{
					"mock": &MockService{},
				},
			})
			tt.setErrHandler(proxy)

			r := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			proxy.ServeHTTP(w, r)

			resp := w.Result()
			if resp.StatusCode != tt.expectStatus {
				t.Errorf("Expected status %d, got %d", tt.expectStatus, resp.StatusCode)
			}
		})
	}
}

func TestTracingMiddleware(t *testing.T) {
	// 初始化测试导出器
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	// 设置全局TracerProvider
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))

	// 创建测试服务
	mockSvc := &MockService{
		backend: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(testResponseBody))
		})),
	}
	defer mockSvc.Close()

	p := NewProxy(&MockRouter{
		services: map[string]service.Service{
			"mock": mockSvc,
		},
	})
	p.tracer = tp.Tracer("test")

	// 创建测试请求
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	// 执行中间件
	p.tracingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证追踪头存在
		if r.Header.Get("traceparent") == "" {
			t.Error("Missing traceparent header")
		}
		// 验证追踪上下文
		ctx := r.Context()
		if span := trace.SpanFromContext(ctx); !span.IsRecording() {
			t.Error("Missing active span in context")
		}
	})).ServeHTTP(rec, req)

	// 验证生成的Span
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	expectedAttributes := []attribute.KeyValue{
		attribute.String("lb.strategy", "round_robin"),
		attribute.Int("backend.count", 1),
	}

	for i, attr := range expectedAttributes {
		if span.Attributes[i].Key != attr.Key {
			t.Errorf("Missing or incorrect attribute %s: %v", attr.Key, span.Attributes[i].Value)
		}
		if span.Attributes[i].Value != attr.Value {
			t.Errorf("Missing or incorrect attribute %s: %v", attr.Key, span.Attributes[i].Value)
		}
	}
}
