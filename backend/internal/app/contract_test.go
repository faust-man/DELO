package app

import (
	"encoding/json"
	"os"
	"testing"
)

func TestContractRoutes(t *testing.T) {
	data, err := os.ReadFile("../../../docs/api/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		OpenAPI string                                `json:"openapi"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err = json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.OpenAPI != "3.1.0" {
		t.Fatal("unsupported contract")
	}
	for name := range resources {
		if _, ok := spec.Paths["/api/v1/"+name]["get"]; !ok {
			t.Errorf("missing list contract: %s", name)
		}
	}
	for _, command := range []string{"login", "logout", "me", "book", "payment", "refund", "allocate", "confirm-rights", "dispatch-artwork", "receive-artwork", "cancel-booking", "move-class", "cancel-class", "complete-class"} {
		if _, ok := spec.Paths["/api/v1/"+command]; !ok {
			t.Errorf("missing command contract: %s", command)
		}
	}
}
