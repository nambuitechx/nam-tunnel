package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/{id}/{path...}", func(w http.ResponseWriter, r *http.Request) {
		if got := proxyPath(r); got != t.Name() {
			// set via request URL below
		}
	})

	cases := []struct {
		url  string
		want string
	}{
		{"/backend", "/"},
		{"/backend/", "/"},
		{"/backend/api/v1/users", "/api/v1/users"},
		{"/backend/api/v1/users?page=1", "/api/v1/users?page=1"},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			mux2 := http.NewServeMux()
			var got string
			h := func(w http.ResponseWriter, r *http.Request) { got = proxyPath(r) }
			mux2.HandleFunc("/{id}/{path...}", h)
			mux2.HandleFunc("/{id}", h)

			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			rec := httptest.NewRecorder()
			mux2.ServeHTTP(rec, req)
			if got != tc.want {
				t.Fatalf("proxyPath(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}
