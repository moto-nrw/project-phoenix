package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestTenantSessionCacheAcrossCommandsAndSchools(t *testing.T) {
	t.Parallel()
	var logins, refreshes, switches atomic.Int32
	api := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/login":
			if logins.Add(1) > 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"code":"identity.mfa_blocked"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"mfa_required","challenge_token":"example-challenge"}`))
		case "/auth/mfa/verify":
			_, _ = w.Write([]byte(`{"access_token":"example-access","refresh_token":"example-refresh"}`))
		case "/auth/refresh":
			refreshes.Add(1)
			require.Contains(t, r.Header.Get("Authorization"), "Bearer example-")
			_, _ = w.Write([]byte(`{"data":{"access_token":"example-rotated-access","refresh_token":"example-rotated-refresh"}}`))
		case "/auth/switch-tenant":
			switches.Add(1)
			_, _ = w.Write([]byte(`{"data":{"access_token":"example-switched-access","refresh_token":"example-switched-refresh"}}`))
		default:
			t.Errorf("unexpected request path: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	defer api.Close()
	dir := filepath.Join(t.TempDir(), "sessions")
	password := testpkg.NewTestPassword()
	for _, school := range []string{"full", "manual", "weekly", "bookings", "full", "manual"} {
		// Each invocation has a new adapter/cache, just like the separate CLI processes.
		adapter := NewCommandAdapter(api.URL, false)
		adapter.tenantSessions = newTenantSessionCache(dir)
		adapter.fetchLatestMFACode = func(context.Context, string, time.Time) (string, error) { return "example-code", nil }
		_, err := adapter.LoginTenant(context.Background(), "admin@example.test", password, school)
		require.NoError(t, err, "status/simulation must reuse MFA proof instead of exhausting code requests")
	}
	require.EqualValues(t, 1, logins.Load())
	require.EqualValues(t, 5, refreshes.Load())
	require.EqualValues(t, 5, switches.Load())
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "one current family per account, not a retired family per school")
	info, err := entries[0].Info()
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestTenantSessionCacheTracksSeederSwitchAndRevokedProof(t *testing.T) {
	t.Parallel()
	var refreshToken string
	var revoked atomic.Bool
	var logins atomic.Int32
	api := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/login":
			logins.Add(1)
			_, _ = w.Write([]byte(`{"access_token":"example-access","refresh_token":"example-refresh"}`))
		case "/auth/switch-tenant":
			_, _ = w.Write([]byte(`{"access_token":"example-target-access","refresh_token":"example-target-refresh"}`))
		case "/auth/refresh":
			if revoked.Load() {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"code":"identity.mfa_required"}`))
				return
			}
			refreshToken = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"access_token":"example-renewed-access","refresh_token":"example-renewed-refresh"}`))
		default:
			t.Errorf("unexpected request path: %s", r.URL.Path)
		}
	})
	defer api.Close()
	dir := t.TempDir()
	password := testpkg.NewTestPassword()
	newAdapter := func() *Adapter {
		a := NewCommandAdapter(api.URL, false)
		a.tenantSessions = newTenantSessionCache(dir)
		return a
	}
	a := newAdapter()
	auth, err := a.LoginTenant(context.Background(), "admin@example.test", password, "source")
	require.NoError(t, err)
	_, _, err = a.Raw(context.Background(), auth, http.MethodPost, "/auth/switch-tenant", map[string]string{"tenant_slug": "target"}, nil)
	require.NoError(t, err)
	_, err = newAdapter().LoginTenant(context.Background(), "admin@example.test", password, "target")
	require.NoError(t, err)
	require.Equal(t, "Bearer example-target-refresh", refreshToken, "never refresh the family retired by the seed profile switch")
	revoked.Store(true)
	_, err = newAdapter().LoginTenant(context.Background(), "admin@example.test", password, "target")
	require.NoError(t, err)
	require.EqualValues(t, 2, logins.Load(), "revoked MFA proof must return to real login")
	_, err = newAdapter().LoginTenant(context.Background(), "admin@example.test", testpkg.NewTestPassword(), "target")
	require.NoError(t, err)
	require.EqualValues(t, 3, logins.Load(), "changed credentials must not reuse the old session")
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	require.NoError(t, err)
	for _, file := range files {
		data, readErr := os.ReadFile(file)
		require.NoError(t, readErr)
		var saved storedTenantSession
		require.NoError(t, json.Unmarshal(data, &saved))
		require.NotEmpty(t, saved.RefreshToken)
	}
}
