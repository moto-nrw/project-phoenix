package api

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// serveGraphFields is the shrink-only field set of the Serve graph (#2745).
// The architecture's composition guard counts fields of the deleted api.API
// only, so this list keeps the graph from growing back into a route
// aggregate: remove an entry when its field goes, never add one. Route
// resources are mounted where they are built.
var serveGraphFields = map[string]struct{}{
	"router": {}, "db": {}, "services": {}, "repos": {}, "tenantRuntime": {},
	"metrics": {}, "tracer": {}, "metricsBearerToken": {}, "feedback": {}, "documents": {},
}

func TestServeGraphFieldsAreShrinkOnly(t *testing.T) {
	t.Parallel()

	graph := reflect.TypeFor[serveGraph]()
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
