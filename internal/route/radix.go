package route

import (
	"net/http"
	"nexus/internal/config"
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
	split   []*config.RouteSplit
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
			if child.part == part {
				current = child
				matched = true
				break
			}
		}
		if !matched {
			child := newNode()
			child.part = part
			child.isWild = part == "*" || part == "**"
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
			return n.findMatchingRoute(req, nil)
		}
		return nil
	}

	// First try exact match
	if exactMatch := n.searchExactPath(path, req); exactMatch != nil {
		return exactMatch
	}

	// Then try wildcard match
	return n.searchWildcardPath(path, req)
}

// searchExactPath Try exact match path
func (n *node) searchExactPath(path string, req *http.Request) *routeInfo {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	current := n

	for _, part := range parts {
		found := false
		for _, child := range current.children {
			if !child.isWild && child.part == part {
				current = child
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}

	if current.isEnd {
		return current.findMatchingRoute(req, nil)
	}

	return nil
}

// searchWildcardPath Try wildcard match path
func (n *node) searchWildcardPath(path string, req *http.Request) *routeInfo {
	// Get all possible wildcard routes
	wildcardRoutes := make([]*routeInfo, 0)

	// Recursively collect all wildcard routes
	n.collectWildcardRoutes("", &wildcardRoutes)

	// Sort by path length, select the longest match
	var bestMatch *routeInfo
	var bestMatchLength int

	for _, info := range wildcardRoutes {
		routePath := info.path

		// Check if it is a wildcard path
		if strings.HasSuffix(routePath, "/*") {
			prefix := strings.TrimSuffix(routePath, "/*")
			if strings.HasPrefix(path, prefix+"/") {
				// Calculate prefix length
				prefixParts := strings.Split(strings.Trim(prefix, "/"), "/")
				if len(prefixParts) > bestMatchLength {
					bestMatchLength = len(prefixParts)
					bestMatch = info
				}
			}
		}

		// If the path is "*", find the matching route from other conditions
		if routePath == "*" {
			bestMatch = n.findMatchingRoute(req, wildcardRoutes)
		}
	}

	if bestMatch != nil {
		return bestMatch
	}

	return nil
}

// collectWildcardRoutes Collect all wildcard routes
func (n *node) collectWildcardRoutes(currentPath string, routes *[]*routeInfo) {
	if n.isWild && n.isEnd {
		for _, info := range n.routeInfos {
			*routes = append(*routes, info)
		}
	}

	for _, child := range n.children {
		newPath := currentPath
		if newPath != "" {
			newPath += "/"
		}
		newPath += child.part
		child.collectWildcardRoutes(newPath, routes)
	}
}

// findMatchingRoute Find matching route information
func (n *node) findMatchingRoute(req *http.Request, routes []*routeInfo) *routeInfo {
	if len(routes) == 0 {
		routes = n.routeInfos
	}

	// If there is only one route information, return directly
	if len(routes) == 1 {
		return routes[0]
	}

	for _, info := range routes {
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
