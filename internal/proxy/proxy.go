package proxy

import (
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"net/url"
	"nexus/internal/balancer"
	"nexus/internal/service"
	"sync"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Proxy struct represents a reverse proxy
type Proxy struct {
	mu sync.RWMutex
	// balancer       balancer.Balancer
	transport      http.RoundTripper
	errorHandler   func(http.ResponseWriter, *http.Request, error)
	tracer         trace.Tracer
	serviceManager *service.ServiceManager
}

// NewProxy creates a new reverse proxy instance
func NewProxy(serviceManager *service.ServiceManager) *Proxy {
	return &Proxy{
		// balancer:  balancer,
		transport:      http.DefaultTransport,
		tracer:         otel.Tracer("nexus.proxy"),
		serviceManager: serviceManager,
	}
}

// ServeHTTP implements the http.Handler interface
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler := http.HandlerFunc(p.handleRequest)
	p.tracingMiddleware(handler).ServeHTTP(w, r)
}

// 新增追踪中间件
func (p *Proxy) tracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// 创建包含负载均衡信息的span
		ctx, span := p.tracer.Start(ctx, "Proxy.Request",
			trace.WithAttributes(
				attribute.String("lb.strategy", p.getBalancerStrategy()),
				attribute.Int("backend.count", p.getBackendCount()),
			))
		defer span.End()

		// 将追踪上下文注入请求
		propagator := otel.GetTextMapPropagator()
		propagator.Inject(ctx, propagation.HeaderCarrier(r.Header))

		// 创建追踪客户端
		traceCtx := httptrace.WithClientTrace(ctx, p.createClientTrace(span))
		r = r.WithContext(traceCtx)

		next.ServeHTTP(w, r)
	})
}

func (p *Proxy) getBalancerStrategy() string {
	// switch p.balancer.(type) {
	switch p.serviceManager.GetService("web").Balancer().(type) {
	case *balancer.RoundRobinBalancer:
		return "round_robin"
	case *balancer.WeightedRoundRobinBalancer:
		return "weighted_round_robin"
	case *balancer.LeastConnectionsBalancer:
		return "least_connections"
	default:
		return "unknown"
	}
}

func (p *Proxy) getBackendCount() int {
	switch b := p.serviceManager.GetService("web").Balancer().(type) {
	case *balancer.RoundRobinBalancer:
		return len(b.GetServers())
	case *balancer.WeightedRoundRobinBalancer:
		return len(b.GetServers())
	case *balancer.LeastConnectionsBalancer:
		return len(b.GetServers())
	default:
		return 0
	}
}

func (p *Proxy) createClientTrace(span trace.Span) *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			span.AddEvent("Acquired connection",
				trace.WithAttributes(
					attribute.Bool("reused", connInfo.Reused),
					attribute.String("remote", connInfo.Conn.RemoteAddr().String()),
				))
		},
	}
}

// handleRequest handles the request
func (p *Proxy) handleRequest(w http.ResponseWriter, r *http.Request) {
	// 选择后端服务器
	target, err := p.serviceManager.GetService("web").NextServer(r.Context())
	// target, err := p.balancer.Next(r.Context())
	if err != nil {
		p.handleError(w, r, err)
		return
	}

	// 解析目标 URL
	targetURL, err := url.Parse(target)
	if err != nil {
		p.handleError(w, r, err)
		return
	}

	// 转发请求
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Transport = otelhttp.NewTransport(http.DefaultTransport)

	proxy.ServeHTTP(w, r)
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
