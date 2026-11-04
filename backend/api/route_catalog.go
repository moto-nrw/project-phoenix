package api

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Route describes one public HTTP route without carrying runtime services.
type Route struct {
	Method  string
	Pattern string
}

//go:embed all:testdata/routes
var routeCatalogFiles embed.FS

// RouteCatalog returns the checked route contract without constructing the
// database-backed Serve runtime. Every resource contributes to the same catalog.
func RouteCatalog() ([]Route, error) { return readRouteCatalog(routeCatalogFiles) }

func readRouteCatalog(source fs.FS) ([]Route, error) {
	var routes []Route
	seen := map[string]bool{}
	err := fs.WalkDir(source, "testdata/routes", func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() || !strings.HasSuffix(filename, ".golden") {
			return fmt.Errorf("invalid route fragment %s", filename)
		}
		fragment, err := readCatalogFragment(source, filename, seen)
		if err != nil {
			return err
		}
		routes = append(routes, fragment...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("route catalog has no routes")
	}
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Method+" "+routes[i].Pattern < routes[j].Method+" "+routes[j].Pattern
	})
	return routes, nil
}

func parseCatalogRoute(line string) (Route, error) {
	method, pattern, ok := strings.Cut(line, " ")
	if !ok || method == "" || !strings.HasPrefix(pattern, "/") || strings.ContainsAny(pattern, " \t\r\n") {
		return Route{}, fmt.Errorf("invalid route catalog line %q", line)
	}
	return Route{Method: method, Pattern: pattern}, nil
}

// Paths, not list positions or counts, determine reviewed-file ownership.
func routeResource(pattern string) string {
	parts := strings.Split(strings.Trim(pattern, "/"), "/")
	if parts[0] == "" {
		return "root/index"
	}
	if len(parts) < 2 || parts[1] == "" || strings.ContainsAny(parts[1], "{*}") {
		return "root/" + parts[0]
	}
	// Parent capabilities live below /parent/me rather than directly below /parent.
	if parts[0] == "parent" && parts[1] == "me" && len(parts) > 2 && parts[2] != "" && !strings.ContainsAny(parts[2], "{*}") {
		return strings.Join(parts[:3], "/")
	}
	return strings.Join(parts[:2], "/")
}

func readCatalogFragment(source fs.FS, filename string, seen map[string]bool) ([]Route, error) {
	data, err := fs.ReadFile(source, filename)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" {
		return nil, fmt.Errorf("empty route fragment %s", filename)
	}
	var routes []Route
	for _, line := range strings.Split(text, "\n") {
		route, err := parseCatalogRoute(line)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filename, err)
		}
		if filename != path.Join("testdata/routes", routeResource(route.Pattern)+".golden") {
			return nil, fmt.Errorf("misplaced route %s in %s", line, filename)
		}
		if seen[line] {
			return nil, fmt.Errorf("duplicate route %s", line)
		}
		seen[line] = true
		routes = append(routes, route)
	}
	return routes, nil
}
