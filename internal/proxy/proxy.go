package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"nexus/internal/balancer"
	"sync"
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
	target, err := p.balancer.Next()
	if err != nil {
		p.handleError(w, r, err)
		return
	}

	targetURL, err := url.Parse(target)
	if err != nil {
		p.handleError(w, r, err)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Transport = p.transport
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
