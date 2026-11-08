package sessiontoken_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken/tokentest"
)

// A token minted for one MFA surface must never be accepted by another
// (#1430 review item #8).

// TestAuthenticateRefreshJWT_RejectsChallengeToken is the end-to-end
// counterpart: a real challenge JWT minted via CreateMFAChallengeJWT is
// sent against a route protected by AuthenticateRefreshJWT, and the
// middleware must answer 401. Pre-fix the response would be 401 anyway
// (because the parser blew up at the missing `id` field), but for the
// wrong reason — an attacker who managed to mint a challenge JWT with
// id/token populated would have slipped through. Post-fix the
// mfa_pending=true gate fires before any of the structural checks.
func TestAuthenticateRefreshJWT_RejectsChallengeToken(t *testing.T) {
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	// Mint a real challenge JWT — what /auth/login returns when MFA is
	// required.
	challengeToken, err := tokentest.CreateMFAChallengeJWT(auth, tokentest.MFAChallengeClaims{
		AccountID: 42,
		Scope:     tokentest.MFAChallengeScopeTenant,
		TenantID:  100,
	}, 5*time.Minute)

	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(sessiontoken.AuthenticateRefreshJWT)
	r.Post("/refresh", func(w http.ResponseWriter, _ *http.Request) {
		// Reached only if the middleware accepts the challenge JWT —
		// which would be a security regression.
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+challengeToken)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"AuthenticateRefreshJWT must reject a challenge JWT — RefreshClaims.ParseClaims is the gate")
}

// TestAuthenticateRefreshJWT_RejectsEnrollmentToken mirrors the previous
// test for an enrollment JWT.
func TestAuthenticateRefreshJWT_RejectsEnrollmentToken(t *testing.T) {
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	enrollmentToken, err := tokentest.CreateMFAEnrollmentJWT(auth, sessiontoken.MFAEnrollmentClaims{
		AccountID: 42,
		Scope:     sessiontoken.MFAEnrollmentScopeTenant,
		TenantID:  100,
	}, 15*time.Minute)

	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(sessiontoken.AuthenticateRefreshJWT)
	r.Post("/refresh", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+enrollmentToken)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"AuthenticateRefreshJWT must reject an enrollment JWT")
}
