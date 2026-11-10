package topology

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func Gateway() http.Handler {
	return gateway(&http.Transport{DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext, ResponseHeaderTimeout: 15 * time.Second, IdleConnTimeout: 60 * time.Second, MaxIdleConnsPerHost: 16})
}

func gateway(transport http.RoundTripper) http.Handler {
	proxies := map[string]*httputil.ReverseProxy{}
	for module := range Modules {
		target := &url.URL{Scheme: "http", Host: "delo-" + module + ":8080"}
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.Transport = transport
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "service unavailable", http.StatusBadGateway)
		}
		proxies[module] = proxy
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			w.WriteHeader(200)
			return
		}
		proxy := proxies[Owner(r.URL.Path)]
		if proxy == nil {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		proxy.ServeHTTP(w, r.WithContext(ctx))
	})
}
