Simple muxer for a Go app.

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
