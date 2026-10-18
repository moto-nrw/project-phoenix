package api

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Preserve the spellings already accepted by the comparison normalizer while
// regenerating actual middleware changes. Raw compiler provenance is not stable
// across builds, and rewriting it obscures the intentional change in the diff.
func preserveGoldenMiddlewareProvenance(t *testing.T, path, got string, normalizers ...func(string) string) string {
	t.Helper()
	if len(normalizers) == 0 {
		return got
	}
	want, err := os.ReadFile(path) // #nosec G304 -- repo-local golden path
	if errors.Is(err, os.ErrNotExist) {
		return got
	}
	require.NoError(t, err)
	return preserveEquivalentMiddlewareNames(string(want), got, normalizers...)
}

func preserveEquivalentMiddlewareNames(want, got string, normalizers ...func(string) string) string {
	normalize := func(name string) string {
		value := "GET / -> " + name + "\n"
		for _, normalize := range normalizers {
			value = normalize(value)
		}
		return value
	}
	oldChains := make(map[string][]string)
	for _, line := range strings.Split(want, "\n") {
		if route, chain, ok := strings.Cut(line, " -> "); ok {
			oldChains[route] = strings.Split(chain, " > ")
		}
	}
	lines := strings.Split(got, "\n")
	for i, line := range lines {
		route, chain, ok := strings.Cut(line, " -> ")
		old, exists := oldChains[route]
		if !ok || !exists {
			continue
		}
		names := strings.Split(chain, " > ")
		for j, name := range names {
			if j < len(old) && normalize(old[j]) == normalize(name) {
				names[j] = old[j]
			}
		}
		lines[i] = route + " -> " + strings.Join(names, " > ")
	}
	return strings.Join(lines, "\n")
}

func TestGoldenUpdatePreservesOnlyEquivalentMiddlewareNames(t *testing.T) {
	t.Parallel()

	want := "GET /kept -> api/common.requirePermissions# > api/iot/compose.iotMetricsMiddleware > api/common.SentrySessionContext\n" +
		"GET /removed -> api.Removed\n" +
		"POST /reordered -> api.First > api.Second\n"
	got := "GET /kept -> api/common.requirePermissions.# > api.iotMetricsMiddleware > api.SentrySessionContext\n" +
		"GET /added -> api.Added\n" +
		"POST /reordered -> api.Second > api.First\n"
	updated := preserveEquivalentMiddlewareNames(want, got, stableMiddlewareTable)
	require.Equal(t, "GET /kept -> api/common.requirePermissions# > api/iot/compose.iotMetricsMiddleware > api.SentrySessionContext\n"+
		"GET /added -> api.Added\n"+
		"POST /reordered -> api.Second > api.First\n", updated)
	require.Equal(t, stableMiddlewareTable(got), stableMiddlewareTable(updated))
}
