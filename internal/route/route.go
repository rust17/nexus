package route

import (
	"net/http"
	"sync"

	"nexus/internal/config"
	"nexus/internal/service"
)

// Router is responsible for matching requests to the corresponding service
type Router interface {
	Match(*http.Request) service.Service
	Update(routes []*config.RouteConfig, services map[string]*config.ServiceConfig) error
}

// Add read-write lock to ensure concurrent safety
type router struct {
	mu       sync.RWMutex
	services map[string]service.Service
	tree     *node
}

// NewRouter Create a new router instance
func NewRouter(routes []*config.RouteConfig, services map[string]*config.ServiceConfig) Router {
	serviceMap := make(map[string]service.Service)
	for name, conf := range services {
		serviceMap[name] = service.NewService(conf)
	}

	r := &router{
		services: serviceMap,
		tree:     buildTree(routes),
	}

	return r
}

// Match Method requires read lock
func (r *router) Match(req *http.Request) service.Service {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routeInfo := r.tree.search(req)
	if routeInfo == nil {
		return nil
	}
	return r.services[routeInfo.service]
}

// Update Implement configuration hot update
func (r *router) Update(routes []*config.RouteConfig, services map[string]*config.ServiceConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Incremental update existing services
	for name, conf := range services {
		if existing, ok := r.services[name]; ok {
			// Reuse existing instance to update configuration
			if err := existing.Update(conf); err != nil {
				return err
			}
		} else {
			// Add new service
			r.services[name] = service.NewService(conf)
		}
	}

	// Clean up deleted services
	for name := range r.services {
		if _, ok := services[name]; !ok {
			delete(r.services, name)
		}
	}

	// Update route tree
	r.tree = buildTree(routes)
	return nil
}

// buildTree Build radix tree
func buildTree(routes []*config.RouteConfig) *node {
	tree := newNode()

	for _, route := range routes {
		tree.insert(route.Match.Path, &routeInfo{
			method:  route.Match.Method,
			host:    route.Match.Host,
			headers: route.Match.Headers,
			service: route.Service,
		})
	}

	return tree
}
