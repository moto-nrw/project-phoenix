package api

import (
	_ "embed"
	"fmt"
	"strings"
)

// Route describes one public HTTP route without carrying runtime services.
type Route struct {
	Method  string
	Pattern string
}

//go:embed testdata/route_table.golden
var routeCatalogText string

// RouteCatalog returns the checked route contract without constructing the
// database-backed Serve runtime.
func RouteCatalog() ([]Route, error) {
	lines := strings.Split(strings.TrimSpace(routeCatalogText), "\n")
	routes := make([]Route, 0, len(lines))
	for lineNumber, line := range lines {
		method, pattern, ok := strings.Cut(line, " ")
		if !ok || method == "" || pattern == "" {
			return nil, fmt.Errorf("invalid route catalog line %d: %q", lineNumber+1, line)
		}
		routes = append(routes, Route{Method: method, Pattern: pattern})
	}
	return routes, nil
}
