package jwt

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests in this file exercise PR #1430 review item #8: every JWT claim
// parser must explicitly reject the FOREIGN MFA-pending flags so a token
// minted for one surface can never be replayed against another. Item #1
// already wired this into AppClaims.ParseClaims; this commit extends the
// defense-in-depth to RefreshClaims, MFAEnrollmentClaims, and
// MFAChallengeClaims so the rejection is universal and symmetric.

// TestRefreshClaims_ParseClaims_RejectsMFAPending proves the
// /auth/refresh route refuses a challenge JWT even if some future change
// adds the `id`/`token` fields the parser would otherwise require.
func TestRefreshClaims_ParseClaims_RejectsMFAPending(t *testing.T) {
	t.Parallel()

	claims := map[string]any{
		"mfa_pending": true,
		"id":          float64(42),
		"token":       "refresh-token-value",
	}
	var c RefreshClaims
	err := c.ParseClaims(claims)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pending-MFA challenge",
		"refresh parser must mention the mfa_pending rejection so the cause is greppable in logs")
}

// TestRefreshClaims_ParseClaims_RejectsMFAEnrollmentPending — same as
// above for the enrollment flag.
func TestRefreshClaims_ParseClaims_RejectsMFAEnrollmentPending(t *testing.T) {
	t.Parallel()

	claims := map[string]any{
		"mfa_enrollment_pending": true,
		"id":                     float64(42),
		"token":                  "refresh-token-value",
	}
	var c RefreshClaims
	err := c.ParseClaims(claims)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pending-MFA-enrollment")
}

// TestMFAEnrollmentClaims_ParseClaims_RejectsMFAPending makes the
// enrollment parser cross-reject a challenge JWT — a JWT cannot satisfy
// BOTH the challenge and the enrollment middleware even if a future bug
// in CreateMFA*JWT ever sets both flags.
func TestMFAEnrollmentClaims_ParseClaims_RejectsMFAPending(t *testing.T) {
	t.Parallel()

	claims := map[string]any{
		"mfa_pending":            true, // foreign flag — must be rejected
		"mfa_enrollment_pending": true,
		"account_id":             float64(42),
		"scope":                  MFAEnrollmentScopeTenant,
	}
	var c MFAEnrollmentClaims
	err := c.ParseClaims(claims)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pending-MFA challenge",
		"enrollment parser must reject tokens carrying the challenge flag — symmetric to MFAChallengeClaims rejecting enrollment_pending")
}

// TestMFAChallengeClaims_ParseClaims_RejectsMFAEnrollmentPending —
// symmetric to the previous test.
func TestMFAChallengeClaims_ParseClaims_RejectsMFAEnrollmentPending(t *testing.T) {
	t.Parallel()

	claims := map[string]any{
		"mfa_pending":            true,
		"mfa_enrollment_pending": true, // foreign flag — must be rejected
		"account_id":             float64(42),
		"scope":                  MFAChallengeScopeTenant,
	}
	var c MFAChallengeClaims
	err := c.ParseClaims(claims)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pending-MFA-enrollment",
		"challenge parser must reject tokens carrying the enrollment flag — symmetric to MFAEnrollmentClaims rejecting mfa_pending")
}

// TestAuthenticateRefreshJWT_RejectsChallengeToken is the end-to-end
// counterpart: a real challenge JWT minted via CreateMFAChallengeJWT is
// sent against a route protected by AuthenticateRefreshJWT, and the
// middleware must answer 401. Pre-fix the response would be 401 anyway
// (because the parser blew up at the missing `id` field), but for the
// wrong reason — an attacker who managed to mint a challenge JWT with
// id/token populated would have slipped through. Post-fix the
// mfa_pending=true gate fires before any of the structural checks.
func TestAuthenticateRefreshJWT_RejectsChallengeToken(t *testing.T) {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	// Mint a real challenge JWT — what /auth/login returns when MFA is
	// required.
	challengeToken, err := auth.CreateMFAChallengeJWT(MFAChallengeClaims{
		AccountID: 42,
		Scope:     MFAChallengeScopeTenant,
		TenantID:  100,
	}, 5*time.Minute)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(AuthenticateRefreshJWT)
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
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	enrollmentToken, err := auth.CreateMFAEnrollmentJWT(MFAEnrollmentClaims{
		AccountID: 42,
		Scope:     MFAEnrollmentScopeTenant,
		TenantID:  100,
	}, 15*time.Minute)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(AuthenticateRefreshJWT)
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
