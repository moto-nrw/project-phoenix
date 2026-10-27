package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestLoginTraeger_SetsUpTheSecondFactorOnTheFirstSignIn(t *testing.T) {
	t.Parallel()

	mailpit := newCodeMailpitServer(t, "traeger@test.de", "424242")
	defer mailpit.Close()

	var confirm map[string]any
	api := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/traeger/auth/login":
			var login map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&login))
			assert.Equal(t, float64(12), login["organization_id"], "the login names the Träger")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "mfa_enrollment_required", "access_token": "enrollment-jwt", "organization_id": 12,
			})
		case "/traeger/auth/mfa/enroll/start":
			assert.Equal(t, "Bearer enrollment-jwt", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]any{"challenge_token": "enroll-challenge"})
		case "/traeger/auth/mfa/enroll/confirm":
			assert.Equal(t, "Bearer enrollment-jwt", r.Header.Get("Authorization"))
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&confirm))
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "traeger-session", "refresh_token": "r"})
		default:
			testpkg.HTTPNotFound(w, r)
		}
	})
	defer api.Close()

	auth, err := newMailpitAdapter(api.URL, mailpit.URL).LoginTraeger(context.Background(), "traeger@test.de", "secret", 12)
	require.NoError(t, err)
	assert.Equal(t, AuthRef{Kind: AuthBearer, Label: "traeger", Token: "traeger-session"}, auth)
	assert.Equal(t, map[string]any{"challenge_token": "enroll-challenge", "code": "424242", "organization_id": float64(12)}, confirm,
		"the confirm redeems the started challenge with the mailed code for the Träger")
}

func TestLoginTraeger_RedeemsTheMailedCodeOfAnEnrolledAccount(t *testing.T) {
	t.Parallel()

	mailpit := newCodeMailpitServer(t, "traeger@test.de", "135790")
	defer mailpit.Close()

	var verify map[string]any
	api := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/traeger/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "mfa_required", "challenge_token": "login-challenge", "masked_email": "t***@test.de", "organization_id": 7,
			})
		case "/traeger/auth/mfa/verify":
			assert.Empty(t, r.Header.Get("Authorization"), "the verify carries no session")
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&verify))
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "traeger-session", "refresh_token": "r"})
		default:
			testpkg.HTTPNotFound(w, r)
		}
	})
	defer api.Close()

	auth, err := newMailpitAdapter(api.URL, mailpit.URL).LoginTraeger(context.Background(), "traeger@test.de", "secret", 0)
	require.NoError(t, err)
	assert.Equal(t, "traeger-session", auth.Token)
	assert.Equal(t, map[string]any{"challenge_token": "login-challenge", "code": "135790", "organization_id": float64(7)}, verify,
		"the verify names the Träger the login resolved")
}

func TestLoginTraeger_RefusesALoginWithoutSessionOrSecondFactor(t *testing.T) {
	t.Parallel()

	api := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "organization_selection_required"})
	})
	defer api.Close()

	_, err := NewCommandAdapter(api.URL, false).LoginTraeger(context.Background(), "traeger@test.de", "secret", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has no access token")
}
