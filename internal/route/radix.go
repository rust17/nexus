package route

import (
	"net/http"
	"regexp"
	"strings"
)

type node struct {
	pattern    string
	part       string
	children   []*node
	isWild     bool
	isEnd      bool
	routeInfos []*routeInfo
}

type routeInfo struct {
	method  string
	host    string
	headers map[string]string
	service string
	path    string
}

func newNode() *node {
	return &node{
		children:   make([]*node, 0),
		routeInfos: make([]*routeInfo, 0),
	}
}

// insert Insert path to radix tree
func (n *node) insert(pattern string, route *routeInfo) {
	pattern = strings.TrimRight(pattern, "/")
	if pattern == "" {
		pattern = "/"
	}
	route.path = pattern

	if pattern == "/" {
		n.pattern = pattern
		n.isEnd = true
		n.routeInfos = append(n.routeInfos, route)
		return
	}

	parts := strings.Split(strings.Trim(pattern, "/"), "/")
	current := n
	for i, part := range parts {
		matched := false
		for _, child := range current.children {
			if child.part == part || child.isWild {
				current = child
				matched = true
				break
			}
		}
		if !matched {
			child := newNode()
			child.part = part
			child.isWild = len(part) > 0 && (part[0] == ':' || part[0] == '*')
			current.children = append(current.children, child)
			current = child
		}

		if i == len(parts)-1 {
			current.pattern = pattern
			current.isEnd = true
			current.routeInfos = append(current.routeInfos, route)
		}
	}
}

// search Search matching route information in radix tree
func (n *node) search(req *http.Request) *routeInfo {
	path := strings.TrimRight(req.URL.Path, "/")
	if path == "" {
		path = "/"
	}

	if path == "/" {
		if n.isEnd {
			return n.findMatchingRoute(req)
		}
		return nil
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	return n.searchParts(parts, 0, req)
}

func (n *node) searchParts(parts []string, height int, req *http.Request) *routeInfo {
	if len(parts) == height || strings.HasPrefix(n.part, "*") {
		if n.isEnd {
			return n.findMatchingRoute(req)
		}
		return nil
	}

	part := parts[height]
	for _, child := range n.children {
		if child.part == part || child.isWild {
			if result := child.searchParts(parts, height+1, req); result != nil {
				return result
			}
		}
	}
	return nil
}

// findMatchingRoute Find matching route information
func (n *node) findMatchingRoute(req *http.Request) *routeInfo {
	// If there is only one route information, return directly
	if len(n.routeInfos) == 1 {
		return n.routeInfos[0]
	}

	for _, info := range n.routeInfos {
		if matchRouteInfo(info, req) {
			return info
		}
	}

	return nil
}

// matchRouteInfo Check if the request matches the route information
func matchRouteInfo(info *routeInfo, req *http.Request) bool {
	// Check HTTP method matching
	if info.method != "" && info.method != req.Method {
		return false
	}

	// Check Host matching
	if info.host != "" && !matchHost(info.host, req.Host) {
		return false
	}

	// Check Header matching
	if len(info.headers) > 0 {
		for header, expectedValue := range info.headers {
			actualValue := req.Header.Get(header)
			if actualValue != expectedValue {
				return false
			}
		}
	}

	return true
}

// matchHost Check if the request Host matches the route configuration
func matchHost(pattern, host string) bool {
	// Handle exact matching
	if pattern == host {
		return true
	}

	// Handle subdomain matching
	if strings.HasPrefix(pattern, "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(host, suffix)
	}

	// Handle regular expression matching
	if strings.HasPrefix(pattern, "^") || strings.HasSuffix(pattern, "$") {
		matched, err := regexp.MatchString(pattern, host)
		if err == nil && matched {
			return true
		}
	}

	return false
}
