/*
Simple muxer without regexp.
Usage example:

	package myapp

	import (
		"net/http"
		muxer "code.google.com/p/go-muxer"
	)

	func handler1(w http.ResponseWriter, r *http.Request, v url.Values) {
		// v will be populated with params from URL path, if any.
		// v.Get("id")
		// v.Get("action")
	}

	func init() {
		m := muxer.NewMux("/api", nil)
		m.Add("GET", "users/{id}", handler1).As("profile")
		m.Add("GET", "products", handler2)
		m.Add("PUT", "products/{id}/do", handler3)
		m.Add("POST", "{domain}/{action}/{id}", handler4).As("whatever")
	}

See muxer_test.go for more.
*/
package muxer

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type Mux interface {
	BasePath() string
	Routes() []*Route
	Add(method string, pattern string, h HandlerFunc) *Route
	BuildPath(routeName string, params ...interface{}) string
	ServeHTTP(w http.ResponseWriter, req *http.Request)
}

// NewMux creates a new muxer and hooks it up with provided http.ServeMux.
// Uses http.DefaultServeMux If httpMux param is nil.
// First param, basePath, is the base for all routes added to this muxer. 
// It can also be zero string, in which case "/" is used as the base path.
// NewMux always prefixes and suffixes provided basePath with "/".
func NewMux(basePath string, httpMux *http.ServeMux) (m Mux) {
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	if !strings.HasSuffix(basePath, "/") {
		basePath = basePath + "/"
	}
	if httpMux == nil {
		httpMux = http.DefaultServeMux
	}
	m = &defaultMux{
		base:    basePath,
		baseLen: len(basePath),
		routes:  make([]*Route, 0),
	}
	httpMux.Handle(basePath, m)
	return
}

// Default implementation of Mux interface
type defaultMux struct {
	base    string
	baseLen int
	routes  []*Route
}

// Returns base path of this mux.
func (dm *defaultMux) BasePath() string {
	return dm.base
}

// Returns the slice of all routes added to this mux.
func (dm *defaultMux) Routes() []*Route {
	return dm.routes
}

// Add a new route to the mux.
func (dm *defaultMux) Add(m string, p string, h HandlerFunc) *Route {
	if len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}
	for _, r := range dm.routes {
		if r.Method == m && r.Pattern == p {
			panic(fmt.Sprintf("Route '%s %s' already exists", m, p))
		}
	}
	route := &Route{
		Method:  m,
		Pattern: p,
		Handler: h,
		mux:     dm,
		parts:   makeParts(p),
	}
	route.partsLen = len(route.parts)
	dm.routes = append(dm.routes, route)
	return route
}

// Generates a path from previously added route pattern extending it with
// provided params
func (dm *defaultMux) BuildPath(name string, params ...interface{}) string {
	var route *Route
	for _, r := range dm.routes {
		if r.Name == name {
			route = r
			break
		}
	}
	if route == nil {
		panic("Route doesn't exist")
	}

	parts := make([]string, 1, route.partsLen+1)
	parts[0] = dm.base
	pi := 0
	for _, rp := range route.parts {
		if rp.isVar {
			parts = append(parts, fmt.Sprintf("%v", params[pi]))
			pi++
		} else {
			parts = append(parts, rp.name)
		}
	}
	return path.Join(parts...)
}

// Matches request URL and hand it over to the route's handler providing it
// with parameters extracted from the URL path (if any).
func (m *defaultMux) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h, v := m.match(req.Method, req.URL.Path[m.baseLen:])
	if h == nil {
		http.NotFound(w, req)
		return
	}
	h(w, req, v)
}

// Looks up a route by matching this mux'es routes againts
// HTTP method (e.g. "GET", "PUT") and URL path. 
// Return Handler of the matched route and parameteres extracted from the URL
// (if any).
func (dm *defaultMux) match(method, path string) (HandlerFunc, url.Values) {
	parts := strings.Split(path, "/")
	partsLen := len(parts)
ROUTES_LOOP:
	for _, r := range dm.routes {
		if r.Method != method || r.partsLen != partsLen {
			continue
		}
		for i, rp := range r.parts {
			if !rp.isVar && rp.name != parts[i] {
				continue ROUTES_LOOP
			}
		}
		// Found a match
		vals := make(url.Values, partsLen)
		for i, rp := range r.parts {
			if rp.isVar {
				vals.Add(rp.name, parts[i])
			}
		}
		return r.Handler, vals
	}
	return nil, nil
}

// Function type that knows how to handle HTTP request, supplied with params
// extracted from a URL path.
type HandlerFunc func(w http.ResponseWriter, r *http.Request, v url.Values)

// Single route struct, element for a mux.Routes()
type Route struct {
	Method  string
	Pattern string
	Handler HandlerFunc
	Name    string
	// Internal
	mux      Mux
	parts    []*pathPart
	partsLen int
}

// Adds a name to this route so that a URL path can be built later on using
// provided name. See BuildPath().
func (r *Route) As(name string) *Route {
	for _, route := range r.mux.Routes() {
		if route.Name == name {
			panic(fmt.Sprintf("Route with name '%s' already exists", name))
		}
	}
	r.Name = name
	return r
}

type pathPart struct {
	isVar bool
	name  string
}

func makeParts(pattern string) []*pathPart {
	split := strings.Split(pattern, "/")
	parts := make([]*pathPart, 0, len(split))
	for _, sp := range split {
		part := &pathPart{isVar: sp[0] == '{' && sp[len(sp)-1] == '}'}
		if part.isVar {
			part.name = sp[1 : len(sp)-1]
		} else {
			part.name = sp
		}
		parts = append(parts, part)
	}
	return parts
}
