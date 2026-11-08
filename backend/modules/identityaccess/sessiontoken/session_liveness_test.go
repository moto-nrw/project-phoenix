package sessiontoken_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken/tokentest"
)

// livenessRouter mounts the production order: Verifier, RequireLiveSession,
// then the route's own Authenticator.
func livenessRouter(t *testing.T, live sessiontoken.SessionLiveness) (*sessiontoken.TokenAuth, http.Handler) {
	t.Helper()
	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)
	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(sessiontoken.RequireLiveSession(live))
	r.With(sessiontoken.Authenticator).Get("/protected", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.With(sessiontoken.AuthenticateRefreshJWT).Post("/refresh", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/public", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	return auth, r
}

func serveWithToken(handler http.Handler, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func accessClaims(family string) sessiontoken.AppClaims {
	return sessiontoken.AppClaims{ID: 42, Sub: "user@example.invalid", Roles: []string{"user"}, TenantID: 5, FamilyID: family}
}

func TestRequireLiveSession(t *testing.T) {
	t.Parallel()

	t.Run("a live family passes and the check sees the token's claims", func(t *testing.T) {
		t.Parallel()
		var seen sessiontoken.AppClaims
		auth, router := livenessRouter(t, func(_ context.Context, claims sessiontoken.AppClaims) (bool, error) {
			seen = claims
			return true, nil
		})
		token, err := tokentest.CreateJWT(auth, accessClaims("family-a"))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, serveWithToken(router, http.MethodGet, "/protected", token).Code)
		assert.Equal(t, "family-a", seen.FamilyID)
		assert.Equal(t, 42, seen.ID)
	})

	t.Run("a revoked family answers 401 although the token is still valid", func(t *testing.T) {
		t.Parallel()
		auth, router := livenessRouter(t, func(context.Context, sessiontoken.AppClaims) (bool, error) { return false, nil })
		token, err := tokentest.CreateJWT(auth, accessClaims("family-a"))
		require.NoError(t, err)
		rr := serveWithToken(router, http.MethodGet, "/protected", token)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		var body sessiontoken.ErrResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		assert.Equal(t, sessiontoken.ErrSessionRevoked.Error(), body.Detail)
		assert.Equal(t, "general.permission", body.Code)
	})

	t.Run("a token without family is refused without asking the store", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		auth, router := livenessRouter(t, func(context.Context, sessiontoken.AppClaims) (bool, error) {
			calls.Add(1)
			return true, nil
		})
		token, err := tokentest.CreateJWT(auth, accessClaims(""))
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, serveWithToken(router, http.MethodGet, "/protected", token).Code)
		assert.Zero(t, calls.Load())
	})

	t.Run("a failed check fails closed with 503", func(t *testing.T) {
		t.Parallel()
		auth, router := livenessRouter(t, func(context.Context, sessiontoken.AppClaims) (bool, error) {
			return false, errors.New("database unavailable")
		})
		token, err := tokentest.CreateJWT(auth, accessClaims("family-a"))
		require.NoError(t, err)
		rr := serveWithToken(router, http.MethodGet, "/protected", token)
		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
		var body sessiontoken.ErrResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		assert.Equal(t, "general.unavailable", body.Code)
	})

	t.Run("an unbound check fails closed", func(t *testing.T) {
		t.Parallel()
		auth, router := livenessRouter(t, nil)
		token, err := tokentest.CreateJWT(auth, accessClaims("family-a"))
		require.NoError(t, err)
		assert.Equal(t, http.StatusServiceUnavailable, serveWithToken(router, http.MethodGet, "/protected", token).Code)
	})

	t.Run("public routes ignore a revoked token", func(t *testing.T) {
		t.Parallel()
		auth, router := livenessRouter(t, func(context.Context, sessiontoken.AppClaims) (bool, error) { return false, nil })
		token, err := tokentest.CreateJWT(auth, accessClaims("family-a"))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, serveWithToken(router, http.MethodGet, "/public", token).Code)
	})

	t.Run("refresh and operator tokens pass to their own checks", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		auth, router := livenessRouter(t, func(context.Context, sessiontoken.AppClaims) (bool, error) {
			calls.Add(1)
			return false, nil
		})
		refresh, err := tokentest.CreateRefreshJWT(auth, tokentest.RefreshClaims{ID: 42, Token: "raw-refresh"})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, serveWithToken(router, http.MethodPost, "/refresh", refresh).Code)

		operator := accessClaims("")
		operator.Scope = "platform"
		token, err := tokentest.CreateJWT(auth, operator)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, serveWithToken(router, http.MethodGet, "/protected", token).Code)
		assert.Zero(t, calls.Load())
	})

	t.Run("requests without a token are untouched", func(t *testing.T) {
		t.Parallel()
		_, router := livenessRouter(t, func(context.Context, sessiontoken.AppClaims) (bool, error) { return false, nil })
		assert.Equal(t, http.StatusOK, serveWithToken(router, http.MethodGet, "/public", "").Code)
		assert.Equal(t, http.StatusOK, serveWithToken(router, http.MethodGet, "/public", "device-api-key").Code)
	})
}
