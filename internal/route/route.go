package route

import (
	"net/http"

	"nexus/internal/config"
	"nexus/internal/service"
)

// Router 负责根据请求匹配对应的服务
type Router interface {
	Match(*http.Request) service.Service
}

type router struct {
	services map[string]service.Service
	tree     *node
}

// NewRouter 创建一个新的路由器实例
func NewRouter(routes []*config.RouteConfig, services map[string]*config.ServiceConfig) Router {
	serviceMap := make(map[string]service.Service)
	for _, serviceConfig := range services {
		serviceMap[serviceConfig.Name] = service.NewService(serviceConfig)
	}

	r := &router{
		services: serviceMap,
		tree:     newNode(),
	}

	// 构建基数树
	for _, route := range routes {
		r.tree.insert(route.Match.Path, &routeInfo{
			method:  route.Match.Method,
			host:    route.Match.Host,
			headers: route.Match.Headers,
			service: route.Service,
		})
	}

	return r
}

// Match 根据请求匹配对应的服务
func (r *router) Match(req *http.Request) service.Service {
	routeInfo := r.tree.search(req)
	if routeInfo == nil {
		return nil
	}
	return r.services[routeInfo.service]
}
