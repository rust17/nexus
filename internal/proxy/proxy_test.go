package proxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

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
	// Initialize test exporter
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	// Set global TracerProvider
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))

	// Create test service
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

	// Create test request
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	// Execute middleware
	p.tracingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify trace header exists
		if r.Header.Get("traceparent") == "" {
			t.Error("Missing traceparent header")
		}
		// Verify trace context
		ctx := r.Context()
		if span := trace.SpanFromContext(ctx); !span.IsRecording() {
			t.Error("Missing active span in context")
		}
	})).ServeHTTP(rec, req)

	// Verify generated span
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
