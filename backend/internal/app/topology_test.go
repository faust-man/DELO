package app

import (
	"delo/internal/topology"
	"testing"
)

func TestEveryResourceHasService(t *testing.T) {
	for resource := range resources {
		if topology.Owner("/api/v1/"+resource) == "" {
			t.Fatalf("resource without service: %s", resource)
		}
	}
	for _, command := range []string{"book", "cancel-booking", "payment", "refund", "dispatch", "move-class", "cancel-class", "complete-class", "relationships", "subscriptions", "waitlist", "attendance", "charges", "allocate", "confirm-rights", "dispatch-artwork", "receive-artwork", "tariffs", "branches", "rooms", "groups"} {
		if topology.Owner("/api/v1/"+command) == "" {
			t.Fatalf("command without service: %s", command)
		}
	}
}
