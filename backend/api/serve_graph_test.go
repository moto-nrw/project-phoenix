package api

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// serveGraphFields is the shrink-only field set of the Serve handler
// (#2745): remove an entry when its field goes, never add one. Route
// resources are mounted where they are built. The settings stand where the
// services were: the weekend plan reads them on every request (#3921).
var serveGraphFields = map[string]struct{}{
	"router": {}, "settings": {},
}

func TestServeGraphFieldsAreShrinkOnly(t *testing.T) {
	t.Parallel()

	graph := reflect.TypeFor[serveHandler]()
	present := make(map[string]struct{}, graph.NumField())
	for field := range graph.Fields() {
		present[field.Name] = struct{}{}
		_, allowed := serveGraphFields[field.Name]
		require.Truef(t, allowed, "serveGraph.%s is new: mount route resources where they are built instead of keeping them on the graph", field.Name)
	}
	for name := range serveGraphFields {
		_, kept := present[name]
		require.Truef(t, kept, "serveGraph.%s is gone: delete its entry from serveGraphFields", name)
	}
}
