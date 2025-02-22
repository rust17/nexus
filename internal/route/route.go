package route

import (
	"net/http"
	"regexp"
	"strings"

	"nexus/internal/config"
	"nexus/internal/service"
)

// Router 负责根据请求匹配对应的服务
type Router interface {
	Match(*http.Request) service.Service
}

type router struct {
	services map[string]service.Service
	routes   []*config.RouteConfig
}

// NewRouter 创建一个新的路由器实例
func NewRouter(routes []*config.RouteConfig, services map[string]*config.ServiceConfig) Router {
	serviceMap := make(map[string]service.Service)
	for _, serviceConfig := range services {
		serviceMap[serviceConfig.Name] = service.NewService(serviceConfig)
	}
	return &router{
		services: serviceMap,
		routes:   routes,
	}
}

// Match 根据请求匹配对应的服务
func (r *router) Match(req *http.Request) service.Service {
	for _, route := range r.routes {
		if r.matchRoute(route, req) {
			return r.services[route.Service]
		}
	}
	return nil
}

// matchRoute 检查请求是否匹配路由规则
func (r *router) matchRoute(route *config.RouteConfig, req *http.Request) bool {
	// 检查 HTTP 方法匹配
	if route.Match.Method != "" && route.Match.Method != req.Method {
		return false
	}

	// 检查 Host 匹配
	if route.Match.Host != "" && !matchHost(route.Match.Host, req.Host) {
		return false
	}

	// 检查 Header 匹配
	if len(req.Header) > 0 && len(route.Match.Headers) == 0 {
		return false
	}
	if len(route.Match.Headers) > 0 {
		for header, expectedValue := range route.Match.Headers {
			actualValue := req.Header.Get(header)
			if actualValue != expectedValue {
				return false
			}
		}
	}

	// 检查路径匹配
	if route.Match.Path != "" && !matchPath(route.Match.Path, req.URL.Path) {
		return false
	}

	return true
}

// matchPath 检查请求路径是否匹配路由配置
func matchPath(pattern, path string) bool {
	// 处理精确匹配
	if pattern == path {
		return true
	}

	// 处理通配符匹配
	if strings.Contains(pattern, "*") {
		regex := strings.Replace(pattern, "*", ".*", -1)
		regex = "^" + regex + "$"
		matched, err := regexp.MatchString(regex, path)
		if err == nil && matched {
			return true
		}
	}

	// 处理正则表达式匹配
	if strings.HasPrefix(pattern, "^") || strings.HasSuffix(pattern, "$") {
		matched, err := regexp.MatchString(pattern, path)
		if err == nil && matched {
			return true
		}
	}

	return false
}

// matchHost 检查请求 Host 是否匹配路由配置
func matchHost(pattern, host string) bool {
	// 处理精确匹配
	if pattern == host {
		return true
	}

	// 处理子域名匹配
	if strings.HasPrefix(pattern, "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(host, suffix)
	}

	// 处理正则表达式匹配
	if strings.HasPrefix(pattern, "^") || strings.HasSuffix(pattern, "$") {
		matched, err := regexp.MatchString(pattern, host)
		if err == nil && matched {
			return true
		}
	}

	return false
}
