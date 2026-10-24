package platformannouncements_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/communication/http/platformannouncements"
)

// TestRouter verifies that the platform router can be constructed.
func TestRouter(t *testing.T) {
	t.Parallel()

	t.Run("router has expected routes", func(t *testing.T) {
		cfg := platformannouncements.ResourceConfig{Runtime: testRuntime()}
		resource := platformannouncements.NewResource(cfg)

		router := resource.Router()
		require.NotNil(t, router)

		routes := router.Routes()
		assert.NotEmpty(t, routes)
	})

	t.Run("every route sits inside the protected group", func(t *testing.T) {
		runtime := testRuntime()
		runtime.Protected = func(router chi.Router, register func(chi.Router)) {
			router.Group(func(r chi.Router) {
				r.Use(func(http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnauthorized)
					})
				})
				register(r)
			})
		}
		router := platformannouncements.NewResource(platformannouncements.ResourceConfig{
			AnnouncementsService: &mockPlatformAnnouncementService{},
			Runtime:              runtime,
		}).Router()

		for _, route := range []struct{ method, path string }{
			{http.MethodGet, "/announcements/unread"},
			{http.MethodGet, "/announcements/unread/count"},
			{http.MethodPost, "/announcements/1/seen"},
			{http.MethodPost, "/announcements/1/dismiss"},
		} {
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, httptest.NewRequest(route.method, route.path, nil))
			assert.Equal(t, http.StatusUnauthorized, rr.Code, route.path)
		}
	})
}
