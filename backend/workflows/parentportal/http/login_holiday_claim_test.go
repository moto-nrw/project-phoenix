package parent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSignIn stands in for the owner's sign-in routes: its /login reports a
// completed sign-in for the address in the body unless refuse is set.
type fakeSignIn struct{ refuse bool }

func (f *fakeSignIn) MountSignIn(r chi.Router, after func(context.Context, string)) {
	r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		if f.refuse {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		after(r.Context(), r.URL.Query().Get("email"))
		w.WriteHeader(http.StatusOK)
	})
}

func (*fakeSignIn) MountSession(chi.Router)          {}
func (*fakeSignIn) MountCredentialChecks(chi.Router) {}

// A completed sign-in hands the address to the workflow, which claims the
// holiday registrations of that address (ADR 0216 point 3); a refused one
// claims nothing.
func TestParentLoginClaimsHolidayRegistrationsOnlyAfterSuccess(t *testing.T) {
	t.Parallel()
	signIn := &fakeSignIn{}
	portal := &fakeParentService{}
	rs := &Resource{ParentService: portal, Auth: LoginRuntime{Routes: signIn}}
	router := rs.RouterWithAuthRateLimiter(nil, func(chi.Router) {})
	login := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/auth/login?email=eva.muster@example.org", strings.NewReader("{}"))
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		return rr
	}

	require.Equal(t, http.StatusOK, login().Code)
	assert.Equal(t, []string{"eva.muster@example.org"}, portal.claimedEmails)

	signIn.refuse = true
	require.NotEqual(t, http.StatusOK, login().Code)
	assert.Len(t, portal.claimedEmails, 1, "a refused sign-in claims nothing")
}
