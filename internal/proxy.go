package internal

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

// Proxy 结构体表示反向代理
type Proxy struct {
	mu           sync.RWMutex
	balancer     Balancer
	transport    http.RoundTripper
	errorHandler func(http.ResponseWriter, *http.Request, error)
}

// NewProxy 创建一个新的反向代理实例
func NewProxy(balancer Balancer) *Proxy {
	return &Proxy{
		balancer:  balancer,
		transport: http.DefaultTransport,
	}
}

// ServeHTTP 实现 http.Handler 接口
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

// SetTransport 设置自定义的 Transport
func (p *Proxy) SetTransport(transport http.RoundTripper) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.transport = transport
}

// SetErrorHandler 设置自定义的错误处理函数
func (p *Proxy) SetErrorHandler(handler func(http.ResponseWriter, *http.Request, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.errorHandler = handler
}

// handleError 处理代理过程中的错误
func (p *Proxy) handleError(w http.ResponseWriter, r *http.Request, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.errorHandler != nil {
		p.errorHandler(w, r, err)
	} else {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
	}
}
