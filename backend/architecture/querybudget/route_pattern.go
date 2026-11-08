package querybudget

import "strings"

// CanonicalRoute removes chi.Walk's synthetic interior mount wildcards.
// chi requires a real wildcard endpoint to end in '*', so an interior '/*/'
// is a composition artifact, not an endpoint selector. Preserve leaf slashes.
func CanonicalRoute(route string) string {
	for strings.Contains(route, "/*/") {
		route = strings.ReplaceAll(route, "/*/", "/")
	}
	return route
}
