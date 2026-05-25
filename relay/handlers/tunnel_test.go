package relay_handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyPath(t *testing.T) {
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
			mux := http.NewServeMux()
			var got string
			h := func(w http.ResponseWriter, r *http.Request) { got = ProxyPath(r) }
			mux.HandleFunc("/{id}/{path...}", h)
			mux.HandleFunc("/{id}", h)

			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if got != tc.want {
				t.Fatalf("ProxyPath(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}
