//go:build testhooks

package account_test

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

// TestAuthRoutesAreReachable fails on a registration a more specific
// pattern shadows: such a handler never runs, whatever it parses (#5549).
func TestAuthRoutesAreReachable(t *testing.T) {
	t.Parallel()
	f := newRBACBudgetFixture(t)
	param := regexp.MustCompile(`\{[^}]+\}`)
	routes := 0
	require.NoError(t, chi.Walk(f.router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		path := param.ReplaceAllString(route, "1")
		resolved := f.router.Find(chi.NewRouteContext(), method, path)
		require.Equal(t, route, resolved, "%s %s is answered by another registration", method, route)
		routes++
		return nil
	}))
	require.Positive(t, routes)
}
