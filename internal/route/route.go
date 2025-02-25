package route

import (
	"net/http"
	"sync"

	"nexus/internal/config"
	"nexus/internal/service"
)

// Router 负责根据请求匹配对应的服务
type Router interface {
	Match(*http.Request) service.Service
	Update(routes []*config.RouteConfig, services map[string]*config.ServiceConfig) error
}

// 添加读写锁保证并发安全
type router struct {
	mu       sync.RWMutex
	services map[string]service.Service
	tree     *node
}

// NewRouter 创建一个新的路由器实例
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

// Match 方法需要加读锁
func (r *router) Match(req *http.Request) service.Service {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routeInfo := r.tree.search(req)
	if routeInfo == nil {
		return nil
	}
	return r.services[routeInfo.service]
}

// Update 实现配置热更新
func (r *router) Update(routes []*config.RouteConfig, services map[string]*config.ServiceConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 增量更新现有服务
	for name, conf := range services {
		if existing, ok := r.services[name]; ok {
			// 复用已有实例更新配置
			if err := existing.Update(conf); err != nil {
				return err
			}
		} else {
			// 新增服务
			r.services[name] = service.NewService(conf)
		}
	}

	// 清理已删除的服务
	for name := range r.services {
		if _, ok := services[name]; !ok {
			delete(r.services, name)
		}
	}

	// 更新路由树
	r.tree = buildTree(routes)
	return nil
}

// buildTree 构建基数树
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
