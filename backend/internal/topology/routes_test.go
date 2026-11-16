package topology

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOwnership(t *testing.T) {
	seen := map[string]bool{}
	for module, routes := range Modules {
		for _, route := range routes {
			if seen[route] {
				t.Fatalf("duplicate route %s", route)
			}
			seen[route] = true
			if Owner("/api/v1/"+route) != module {
				t.Fatal(route)
			}
			for other := range Modules {
				h := Scope(other, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
				rr := httptest.NewRecorder()
				h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/"+route, nil))
				want := 404
				if other == module {
					want = 204
				}
				if rr.Code != want {
					t.Fatalf("%s on %s: %d", route, other, rr.Code)
				}
			}
		}
	}
	for _, path := range []string{"/api/v2/persons", "/api/v1/persons/extra", "/api/v1/unknown"} {
		if Owner(path) != "" {
			t.Fatal(path)
		}
	}
}

func TestGatewayUnknownAndHealth(t *testing.T) {
	for path, want := range map[string]int{"/healthz": 200, "/readyz": 200, "/api/v1/unknown": 404} {
		rr := httptest.NewRecorder()
		Gateway().ServeHTTP(rr, httptest.NewRequest("GET", path, nil))
		if rr.Code != want {
			t.Fatal(path, rr.Code)
		}
	}
}
