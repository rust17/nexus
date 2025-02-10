package proxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	lb "nexus/internal/balancer"

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

func TestProxy_RequestFlow(t *testing.T) {
	tests := []struct {
		name           string
		backendHandler http.HandlerFunc
		expectStatus   int
		expectBody     string
	}{
		{
			name: "HealthyBackend",
			backendHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(testResponseBody))
			},
			expectStatus: http.StatusOK,
			expectBody:   testResponseBody,
		},
		{
			name:           "NoBackendAvailable",
			backendHandler: nil,
			expectStatus:   http.StatusServiceUnavailable,
			expectBody:     "Service unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var backend *httptest.Server
			if tt.backendHandler != nil {
				backend = httptest.NewServer(tt.backendHandler)
				defer backend.Close()
			}

			balancer := lb.NewBalancer("round_robin")
			if backend != nil {
				balancer.Add(backend.URL)
			}

			proxy := NewProxy(balancer)

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			proxy.ServeHTTP(w, req)

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
	balancer := lb.NewBalancer("round_robin")
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.Write([]byte(testResponseBody))
	}))
	defer backend.Close()
	balancer.Add(backend.URL)

	proxy := NewProxy(balancer)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			proxy.ServeHTTP(w, req)
		}()
	}
	wg.Wait()
}

func TestProxy_CustomTransport(t *testing.T) {
	balancer := lb.NewBalancer("round_robin")
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testResponseBody))
	}))
	defer backend.Close()
	balancer.Add(backend.URL)

	proxy := NewProxy(balancer)

	customTransport := &http.Transport{
		ResponseHeaderTimeout: 1 * time.Second,
	}
	proxy.SetTransport(customTransport)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	if proxy.transport != customTransport {
		t.Error("Custom transport not set properly")
	}
}

func TestProxy_ErrorHandler(t *testing.T) {
	tests := []struct {
		name         string
		setupHandler func(*Proxy)
		expectStatus int
	}{
		{
			name:         "DefaultHandler",
			setupHandler: func(p *Proxy) {},
			expectStatus: http.StatusServiceUnavailable,
		},
		{
			name: "CustomHandler",
			setupHandler: func(p *Proxy) {
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
			balancer := lb.NewBalancer("round_robin")
			proxy := NewProxy(balancer)
			tt.setupHandler(proxy)

			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			proxy.ServeHTTP(w, req)

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

	// 创建测试代理
	b := lb.NewRoundRobinBalancer()
	b.Add("http://backend1")
	p := NewProxy(b)
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
