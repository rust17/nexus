package proxy

import (
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"net/url"
	"nexus/internal/balancer"
	"nexus/internal/route"
	"sync"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Proxy struct represents a reverse proxy
type Proxy struct {
	mu           sync.RWMutex
	router       route.Router
	transport    http.RoundTripper
	errorHandler func(http.ResponseWriter, *http.Request, error)
	tracer       trace.Tracer
}

// NewProxy creates a new reverse proxy instance
func NewProxy(router route.Router) *Proxy {
	return &Proxy{
		router:    router,
		transport: http.DefaultTransport,
		tracer:    otel.Tracer("nexus.proxy"),
	}
}

// ServeHTTP implements the http.Handler interface
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler := http.HandlerFunc(p.handleRequest)
	p.tracingMiddleware(handler).ServeHTTP(w, r)
}

// Add tracing middleware
func (p *Proxy) tracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		service := p.router.Match(r)

		// Create span with load balancer information
		ctx, span := p.tracer.Start(ctx, "Proxy.Request",
			trace.WithAttributes(
				attribute.String("lb.strategy", p.getBalancerStrategy(service.Balancer())),
				attribute.Int("backend.count", p.getBackendCount(service.Balancer())),
			))
		defer span.End()

		// Inject tracing context into request
		propagator := otel.GetTextMapPropagator()
		propagator.Inject(ctx, propagation.HeaderCarrier(r.Header))

		// Create tracing client
		traceCtx := httptrace.WithClientTrace(ctx, p.createClientTrace(span))
		r = r.WithContext(traceCtx)

		next.ServeHTTP(w, r)
	})
}

func (p *Proxy) getBalancerStrategy(b balancer.Balancer) string {
	switch b.(type) {
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

func (p *Proxy) getBackendCount(b balancer.Balancer) int {
	switch b := b.(type) {
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
	// Select backend server
	service := p.router.Match(r)
	target, err := service.NextServer(r.Context())

	if err != nil {
		p.handleError(w, r, err)
		return
	}

	// Parse target URL
	targetURL, err := url.Parse(target)
	if err != nil {
		p.handleError(w, r, err)
		return
	}

	// Forward request
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
