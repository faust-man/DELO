// Package topology defines the public route ownership shared by gateway and services.
package topology

import (
	"net/http"
	"strings"
)

var Modules = map[string][]string{
	"identity":      {"login", "logout", "me"},
	"contacts":      {"persons", "relationships"},
	"organizations": {"branches", "rooms", "groups", "organization_policies"},
	"education":     {"classes", "class_changes", "move-class", "cancel-class", "complete-class"},
	"bookings":      {"bookings", "book", "cancel-booking", "cancellation_events", "waitlist", "attendance", "attendance_events"},
	"subscriptions": {"tariffs", "subscriptions"},
	"finance":       {"payments", "payment", "refunds", "refund", "charges", "allocate", "allocations"},
	"artworks":      {"artworks", "confirm-rights", "rights_confirmations"},
	"inventory":     {"movements", "dispatch", "dispatch-artwork", "receive-artwork", "artwork_receipts"},
	"exhibitions":   {"exhibitions"},
	"reports":       {"finance_summary"},
	"audit":         {"audit", "operational_tasks"},
	"notifications": {"notifications"},
}

func Owner(path string) string {
	if !strings.HasPrefix(path, "/api/v1/") {
		return ""
	}
	route := strings.TrimPrefix(path, "/api/v1/")
	for module, routes := range Modules {
		for _, candidate := range routes {
			if route == candidate {
				return module
			}
		}
	}
	return ""
}

// Scope prevents direct calls to a service from accessing another module.
func Scope(module string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && r.URL.Path != "/readyz" && Owner(r.URL.Path) != module {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
