package querybudget

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRoutesUseSortedDeduplicatedSets(t *testing.T) {
	t.Parallel()
	var routes Routes
	if err := json.Unmarshal([]byte(`["PUT /b","GET /a","GET /a"]`), &routes); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(routes, Routes{"GET /a", "PUT /b"}) {
		t.Fatalf("routes = %q", routes)
	}
	for _, bad := range []string{`null`, `[""]`, `42`, `["GET /a",null]`} {
		if err := json.Unmarshal([]byte(bad), &routes); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}

func TestCanonicalRouteMatchesWalkMountArtifacts(t *testing.T) {
	t.Parallel()
	for before, after := range map[string]string{
		"GET /api/iot/*/type/{type}": "GET /api/iot/type/{type}",
		"GET /a/*/*/b/":              "GET /a/b/",
		"GET /files/*":               "GET /files/*",
	} {
		if got := CanonicalRoute(before); got != after {
			t.Fatalf("%q -> %q, want %q", before, got, after)
		}
	}
}
