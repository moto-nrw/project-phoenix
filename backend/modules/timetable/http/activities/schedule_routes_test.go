package httpadapter

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func TestActivityScheduleRoutesAreReadOnly(t *testing.T) {
	t.Parallel()
	resource := &Resource{}
	router := resource.Router(func(r chi.Router, mount func(chi.Router, func(http.Handler) http.Handler)) {
		mount(r, func(next http.Handler) http.Handler { return next })
	})
	routes := make(map[string]bool)
	require.NoError(t, chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes[method+" "+route] = true
		return nil
	}))
	require.True(t, routes["GET /{id}/schedules"])
	require.True(t, routes["GET "+routeScheduleByID])
	for _, route := range []string{
		"POST /{id}/schedules", "PUT " + routeScheduleByID,
		"DELETE " + routeScheduleByID, "GET /schedules/available",
	} {
		t.Run(route, func(t *testing.T) {
			require.False(t, routes[route], "unused schedule endpoint must not be exposed: %s", route)
		})
	}
}
