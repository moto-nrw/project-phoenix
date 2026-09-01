package jwt

// Tests for the MFAEnrollmentAuthenticator middleware. These are the
// "Tests: Enrollment-Token darf keine geschützten Routen aufrufen"
// requirement from Item #1 of the #1430 review — they prove that
//
//   (a) an enrollment-scoped token authorizes the enroll routes, AND
//   (b) the same token is REJECTED by the regular Authenticator, AND
//   (c) a regular access token is REJECTED by the enrollment authenticator.
//
// Together those three properties pin the security boundary the review
// asked for.

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// (a) enrollment token reaches the enroll handler and the claims arrive
// on the request context for handler use.
func TestMFAEnrollmentAuthenticator_AcceptsEnrollmentToken(t *testing.T) {
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	tokenString, err := auth.CreateMFAEnrollmentJWT(MFAEnrollmentClaims{
		AccountID: 4242,
		Scope:     MFAEnrollmentScopeTenant,
		TenantID:  70010001,
	}, 5*time.Minute)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(MFAEnrollmentAuthenticator)
	r.Post("/mfa/enroll/start", func(w http.ResponseWriter, r *http.Request) {
		claims, ok := EnrollmentClaimsFromCtx(r.Context())
		assert.True(t, ok, "enrollment claims must be set on the context for downstream handlers")
		assert.Equal(t, int64(4242), claims.AccountID)
		assert.Equal(t, int64(70010001), claims.TenantID)
		assert.Equal(t, MFAEnrollmentScopeTenant, claims.Scope)
		assert.True(t, claims.MFAEnrollmentPending)
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/mfa/enroll/start", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code,
		"enrollment-scoped JWT must authorize the enroll endpoint")
}

// (b) the same enrollment token is rejected by the regular Authenticator,
// which protects every fully-authenticated route in the API. This is the
// boundary the #1430 review demanded: an enrollment token must NEVER be
// usable as a session token.
func TestRegularAuthenticator_RejectsEnrollmentToken(t *testing.T) {
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	tokenString, err := auth.CreateMFAEnrollmentJWT(MFAEnrollmentClaims{
		AccountID: 4242,
		Scope:     MFAEnrollmentScopeTenant,
	}, 5*time.Minute)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(Authenticator)
	called := false
	r.Get("/protected", func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"enrollment token must NOT authorize a regular session route — this is the Item #1 bypass fix")
	assert.False(t, called, "the handler must not run when authentication is rejected")
}

// (c) inverse: a regular access token (no mfa_enrollment_pending claim)
// is rejected by the enrollment-only authenticator. This guards against
// a leaked access token being replayed at /auth/mfa/enroll/* to start a
// fresh email-code flow under someone else's identity.
func TestMFAEnrollmentAuthenticator_RejectsRegularAccessToken(t *testing.T) {
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	access, err := auth.CreateJWT(AppClaims{
		ID:    4242,
		Sub:   "account:4242",
		Roles: []string{"admin"},
	})
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(MFAEnrollmentAuthenticator)
	called := false
	r.Post("/mfa/enroll/start", func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/mfa/enroll/start", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"a regular access token must NOT authorize the enroll surface — the two scopes are disjoint")
	assert.False(t, called)
}

func TestMFAEnrollmentAuthenticator_NoToken(t *testing.T) {
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(MFAEnrollmentAuthenticator)
	r.Post("/mfa/enroll/start", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/mfa/enroll/start", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMFAEnrollmentAuthenticator_ExpiredToken(t *testing.T) {
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	tokenString, err := auth.CreateMFAEnrollmentJWT(MFAEnrollmentClaims{
		AccountID: 4242,
		Scope:     MFAEnrollmentScopeTenant,
	}, time.Millisecond)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(MFAEnrollmentAuthenticator)
	r.Post("/mfa/enroll/start", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/mfa/enroll/start", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"expired enrollment tokens must be rejected, just like expired access tokens")
}
