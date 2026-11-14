package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Many families open the school's online enrollment from one address at once,
// like the holiday link (#6739). Its public routes have the limit of that
// link, apart from the general limiter; the school's own enrollment routes
// stay on the general one.
func TestPublicEnrollmentRoutesHaveTheLimitOfThePublicLink(t *testing.T) {
	t.Parallel()
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	router := chi.NewRouter()
	setupRateLimiting(router, nil, true, false)
	router.Get("/api/enrollment/phases/public/{tenantSlug}", ok)
	router.Post("/api/enrollment/{tenantSlug}/submit", ok)
	router.Get("/api/enrollment/admin/requests", ok)
	send := func(method, path string) int {
		request := httptest.NewRequest(method, path, nil)
		request.RemoteAddr = "192.0.2.10:1234"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response.Code
	}

	_, burst := publicLinkLimits(generalRateLimitFromEnv())
	for i := range burst {
		require.Equal(t, http.StatusOK, send(http.MethodGet, "/api/enrollment/phases/public/schule"), "read %d of the burst of %d", i+1, burst)
	}
	assert.Equal(t, http.StatusTooManyRequests, send(http.MethodGet, "/api/enrollment/phases/public/schule"), "the read past the burst of the link")
	assert.Equal(t, http.StatusOK, send(http.MethodPost, "/api/enrollment/schule/submit"), "writes have a budget of their own")
	assert.Equal(t, http.StatusOK, send(http.MethodGet, "/api/enrollment/admin/requests"), "the general limiter did not count the public reads")

	_, generalBurst := generalRateLimitFromEnv()
	for range generalBurst {
		send(http.MethodGet, "/api/enrollment/admin/requests")
	}
	assert.Equal(t, http.StatusTooManyRequests, send(http.MethodGet, "/api/enrollment/admin/requests"), "the school's routes stay on the general limiter")
}

func TestPublicEnrollmentRequestsAreTheParentFacingRoutes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		method, path string
		public       bool
	}{
		{http.MethodGet, "/api/enrollment/phases/public/schule", true},
		{http.MethodGet, "/api/enrollment/form-bootstrap/public/schule/7", true},
		{http.MethodGet, "/api/enrollment/legal/schule", true},
		{http.MethodGet, "/api/enrollment/legal/schule/7", true},
		{http.MethodPost, "/api/enrollment/schule/submit", true},
		{http.MethodPost, "/api/enrollment/schule/uploads", true},
		{http.MethodGet, "/api/enrollment/requests/token", true},
		{http.MethodPut, "/api/enrollment/requests/token", true},
		{http.MethodPost, "/api/enrollment/requests/token/change-requests", true},
		{http.MethodGet, "/api/enrollment/phases", false},
		{http.MethodGet, "/api/enrollment/phases/7", false},
		{http.MethodGet, "/api/enrollment/me/profile", false},
		{http.MethodGet, "/api/enrollment/admin/requests/7/uploads/3", false},
		{http.MethodGet, "/api/enrollment/schule/submit", false},
		{http.MethodGet, "/api/enrollmentx/phases/public/schule", false},
		{http.MethodGet, "/enrollment/phases/public/schule", false},
	} {
		assert.Equal(t, tc.public, isPublicEnrollmentRequest(httptest.NewRequest(tc.method, tc.path, nil)), "%s %s", tc.method, tc.path)
	}
}
