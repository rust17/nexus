package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"nexus/internal/balancer"
	"sync"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// Proxy struct represents a reverse proxy
type Proxy struct {
	mu           sync.RWMutex
	balancer     balancer.Balancer
	transport    http.RoundTripper
	errorHandler func(http.ResponseWriter, *http.Request, error)
}

// NewProxy creates a new reverse proxy instance
func NewProxy(balancer balancer.Balancer) *Proxy {
	return &Proxy{
		balancer:  balancer,
		transport: http.DefaultTransport,
	}
}

// ServeHTTP implements the http.Handler interface
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("nexus").Start(r.Context(), "Proxy.Request")
	if span != nil {
		defer span.End()
	}

	// 记录请求属性
	if span != nil {
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
			attribute.String("http.user_agent", r.UserAgent()),
		)
	}

	// 选择后端服务器
	target, err := p.balancer.Next()
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		p.handleError(w, r, err)
		return
	}

	// 解析目标 URL
	targetURL, err := url.Parse(target)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		p.handleError(w, r, err)
		return
	}

	// 转发请求
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Transport = otelhttp.NewTransport(http.DefaultTransport)

	// 记录目标服务器
	if span != nil {
		span.SetAttributes(attribute.String("backend.target", target))
	}

	proxy.ServeHTTP(w, r.WithContext(ctx))
}

// SetTransport sets a custom Transport
func (p *Proxy) SetTransport(transport http.RoundTripper) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.transport = transport
}

// SetErrorHandler sets a custom error handler function
func (p *Proxy) SetErrorHandler(handler func(http.ResponseWriter, *http.Request, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.errorHandler = handler
}

// handleError handles errors during the proxy process
func (p *Proxy) handleError(w http.ResponseWriter, r *http.Request, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.errorHandler != nil {
		p.errorHandler(w, r, err)
	} else {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
	}
}
