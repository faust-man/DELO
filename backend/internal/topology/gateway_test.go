package topology

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGatewayPreservesSecurityAndCommandContext(t *testing.T) {
	for module, routes := range Modules {
		calls := 0
		h := gateway(transportFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.URL.Host != "delo-"+module+":8080" || r.URL.RawQuery != "page=1" {
				t.Fatalf("wrong upstream: %s", r.URL)
			}
			for key, want := range map[string]string{"Origin": "https://studio.example", "X-Organization-ID": "tenant-a", "Idempotency-Key": "repeat-safe-key", "Cookie": "delo_session=secret"} {
				if r.Header.Get(key) != want {
					t.Fatalf("lost %s", key)
				}
			}
			body, _ := io.ReadAll(r.Body)
			if string(body) != `{"amount":100}` {
				t.Fatal("changed command body")
			}
			return &http.Response{StatusCode: 409, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":"conflict"}`))}, nil
		}))
		request := httptest.NewRequest("POST", "/api/v1/"+routes[0]+"?page=1", strings.NewReader(`{"amount":100}`))
		for key, value := range map[string]string{"Origin": "https://studio.example", "X-Organization-ID": "tenant-a", "Idempotency-Key": "repeat-safe-key", "Cookie": "delo_session=secret"} {
			request.Header.Set(key, value)
		}
		out := httptest.NewRecorder()
		h.ServeHTTP(out, request)
		if calls != 1 || out.Code != 409 {
			t.Fatalf("calls=%d status=%d", calls, out.Code)
		}
	}
}

func TestGatewayDoesNotRetryFailedMutations(t *testing.T) {
	calls := 0
	h := gateway(transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("private connection details")
	}))
	out := httptest.NewRecorder()
	h.ServeHTTP(out, httptest.NewRequest("POST", "/api/v1/payment", strings.NewReader(`{}`)))
	if calls != 1 || out.Code != 502 || strings.Contains(out.Body.String(), "private") {
		t.Fatalf("unexpected failure: calls=%d %s", calls, out.Body.String())
	}
}
