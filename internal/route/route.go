package route

import (
	"math/rand"
	"net/http"
	"sync"
	"time"

	"nexus/internal/config"
	"nexus/internal/service"
)

var (
	// 为分流功能创建一个全局随机数生成器
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
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

	if len(routeInfo.split) > 0 {
		// Handle split routing based on weights
		return r.services[r.selectServiceBySplit(routeInfo)]
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
			split:   route.Split,
		})
	}

	return tree
}

// selectServiceBySplit selects a service based on the configured weights
func (r *router) selectServiceBySplit(routeInfo *routeInfo) string {
	// If there's only one split entry, return it directly
	if len(routeInfo.split) == 1 {
		return routeInfo.split[0].Service
	}

	// Calculate total weight
	totalWeight := 0
	for _, split := range routeInfo.split {
		totalWeight += split.Weight
	}

	// If total weight is 0, return the first service (should not happen)
	if totalWeight == 0 {
		return routeInfo.split[0].Service
	}

	// Generate a random number between 0 and totalWeight
	randomWeight := rng.Intn(totalWeight)

	// Select service based on weight
	currentWeight := 0
	for _, split := range routeInfo.split {
		currentWeight += split.Weight
		if randomWeight < currentWeight {
			return split.Service
		}
	}

	// Fallback to the first service (should not happen)
	return routeInfo.split[0].Service
}
