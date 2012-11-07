// Muxer tests
// 
// +build !appengine
package muxer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

var dummy = func(w http.ResponseWriter, r *http.Request, v url.Values) {
	fmt.Fprintf(w, "params:%s", v.Encode())
}

func assertEqual(t *testing.T, actual, expected string) {
	if actual != expected {
		t.Fatalf("Expected '%s' got '%s'", expected, actual)
	}
}

func TestAddRoute(t *testing.T) {
	m := NewMux("", http.NewServeMux())
	r := m.Add("GET", "products/{action}/{id}", dummy)
	assertEqual(t, r.Method, "GET")
	assertEqual(t, r.Pattern, "products/{action}/{id}")
	assertEqual(t, r.Name, "")
	if r.Handler == nil {
		t.Fatalf("Expected dummy handler, got nil")
	}
	if r.mux != m {
		t.Fatalf("Expected original muxer, got %v", r.mux)
	}

	m2 := NewMux("/api", http.NewServeMux())
	r2 := m2.Add("PUT", "/scores/{id}", dummy)
	assertEqual(t, r2.Method, "PUT")
	assertEqual(t, r2.Pattern, "scores/{id}")

	defer func() {
		if err := recover(); err == nil {
			t.Fatalf("Expected panic, got no error instead")
		}
	}()
	// should panic because of the same method + pattern
	m2.Add("PUT", "/scores/{id}", dummy)
}

func TestNamedRoute(t *testing.T) {
	m := NewMux("/api", http.NewServeMux())
	m.Add("POST", "users/{id}", dummy)
	r := m.Add("GET", "users/{id}", dummy).As("profile")
	assertEqual(t, r.Name, "profile")
	routesLen := len(m.Routes())
	if routesLen != 2 {
		t.Fatalf("Expected 2 routes in the slice, got %d", routesLen)
	}

	defer func() {
		if err := recover(); err == nil {
			t.Fatalf("Expected panic, got no error instead")
		}
	}()
	// should panic because of the same route name
	m.Add("PUT", "users/{id}", dummy).As("profile")
}

func TestServeHttp(t *testing.T) {
	h := http.NewServeMux()
	NewMux("/api", h).Add("GET", "users/{action}/{id}", dummy)

	// Test 200 OK
	req, err := http.NewRequest("GET", "/api/users/show/alex", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("Expected 200 OK, got %d", w.Code)
	}
	resp := w.Body.String()
	if strings.Index(resp, "action=show") < 0 {
		t.Fatalf("Expected action param in response body '%s'", resp)
	}
	if strings.Index(resp, "id=alex") < 0 {
		t.Fatalf("Expected id param in response body '%s'", resp)
	}

	// Test 404 Not Found
	req, err = http.NewRequest("GET", "/something/else", nil)
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("Expected 404 Not found, got %d", w.Code)
	}
}

//////////////////////////////////////////////////////////////////////////////
// Examples

func ExampleBasePath() {
	for i, bp := range []string{"", "/", "/api", "base/", "/both/"} {
		m := NewMux(bp, http.NewServeMux())
		fmt.Printf("%d: %s\n", i+1, m.BasePath())
	}
	// Output:
	// 1: /
	// 2: /
	// 3: /api/
	// 4: /base/
	// 5: /both/
}

func ExampleRouteBuilding() {
	m := NewMux("/api", http.NewServeMux())
	m.Add("GET", "users/{id}", dummy).As("profile")
	m.Add("GET", "/products", dummy).As("list")
	m.Add("PUT", "products/{id}/do", dummy).As("product")
	m.Add("POST", "{domain}/{action}/{id}", dummy).As("whatever")

	fmt.Println(m.BuildPath("profile", 123))
	fmt.Println(m.BuildPath("list"))
	fmt.Println(m.BuildPath("product", "pxyz"))
	fmt.Println(m.BuildPath("whatever", "somedomain", true, 23.45))
	// Output:
	// /api/users/123
	// /api/products
	// /api/products/pxyz/do
	// /api/somedomain/true/23.45
}

//////////////////////////////////////////////////////////////////////////////
// Benchmarks

func buildMuxForBench(sm *http.ServeMux) (m Mux) {
	if sm == nil {
		sm = http.NewServeMux()
	}
	m = NewMux("/api", sm)
	m.Add("GET", "users/{id}", dummy).As("profile")
	m.Add("GET", "products", dummy).As("list")
	m.Add("PUT", "products/{id}/do", dummy).As("product")
	m.Add("GET", "{domain}/{action}/{id}", dummy)
	m.Add("POST", "{domain}/{action}/{id}", dummy).As("whatever")
	return
}

func BenchmarkRouteMatch(b *testing.B) {
	b.StopTimer()
	m := buildMuxForBench(nil).(*defaultMux)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		m.match("PUT", "/api/products/321/do")
	}
}

func BenchmarkServe200(b *testing.B) {
	b.StopTimer()
	h := http.NewServeMux()
	buildMuxForBench(h)
	req, err := http.NewRequest("GET", "/api/whatever/show/me", nil)
	if err != nil {
		panic(err)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			panic(fmt.Sprintf("Expected 200 OK, got %d", w.Code))
		}
	}
}

func BenchmarkServe404(b *testing.B) {
	b.StopTimer()
	h := http.NewServeMux()
	buildMuxForBench(h)
	req, err := http.NewRequest("GET", "/something/that/doesnt/exist", nil)
	if err != nil {
		panic(err)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 404 {
			panic(fmt.Sprintf("Expected 404, got %d", w.Code))
		}
	}
}

func BenchmarkRouteBuild(b *testing.B) {
	b.StopTimer()
	m := buildMuxForBench(nil)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		m.BuildPath("whatever", "somedomain", true, 23.45)
	}
}
