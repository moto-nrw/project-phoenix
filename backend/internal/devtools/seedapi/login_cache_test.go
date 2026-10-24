package api

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginCachingAdapterRenewsLoginAfterTenantSwitch(t *testing.T) {
	t.Parallel()
	var logins int
	var retired string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/login":
			logins++
			_, _ = fmt.Fprintf(w, `{"data":{"access_token":"verified-session-%d"}}`, logins)
		case "/auth/switch-tenant":
			var body struct {
				TenantSlug string `json:"tenant_slug"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(seedHTTPStatusInternalServerError)
				return
			}
			if body.TenantSlug == "unmapped" {
				w.WriteHeader(seedHTTPStatusUnauthorized)
				_, _ = fmt.Fprint(w, `{"status":401,"code":"identity.tenant_access_denied"}`)
				return
			}
			if r.Header.Get("Authorization") == retired {
				w.WriteHeader(seedHTTPStatusUnauthorized)
				_, _ = fmt.Fprint(w, `{"status":401,"code":"general.permission","detail":"invalid token format"}`)
				return
			}
			// Model the old family's elapsed retirement grace before the next login.
			retired = r.Header.Get("Authorization")
			_, _ = fmt.Fprint(w, `{"access_token":"switched-session"}`)
		default:
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()
	adapter := newLoginCachingAdapter(newSeedTestAdapter(srv.URL))
	client := NewClientWithAdapter(adapter, false)
	ctx := t.Context()
	first, err := adapter.LoginTenant(ctx, "admin@example.test", "secret", "primary")
	require.NoError(t, err)
	_, err = client.PostWithAuth(first, "/auth/switch-tenant", map[string]string{"tenant_slug": "unmapped"})
	require.Error(t, err)
	first, err = adapter.LoginTenant(ctx, "admin@example.test", "secret", "primary")
	require.NoError(t, err)
	require.Equal(t, 1, logins, "a denied switch leaves the original session alive")
	_, err = client.PostWithAuth(first, "/auth/switch-tenant", map[string]string{"tenant_slug": "manual"})
	require.NoError(t, err)
	// Marketing signs into primary again before switching to another organization.
	fresh, err := adapter.LoginTenant(ctx, "admin@example.test", "secret", "primary")
	require.NoError(t, err)
	raw, err := client.PostWithAuth(fresh, "/auth/switch-tenant", map[string]string{"tenant_slug": "marketing"})
	require.NoError(t, err, "a repeated login must not reuse the retired MFA session")
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.Unmarshal(raw, &tokens))
	require.NotEmpty(t, tokens.AccessToken)
	require.Equal(t, 2, logins)
}

func TestLoginCachingAdapterReusesTokenPerAccount(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	logins := map[string]int{}
	failNext := true
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		mu.Lock()
		defer mu.Unlock()
		logins[r.URL.Path]++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/parent/auth/login" && failNext {
			failNext = false
			w.WriteHeader(401)
			_, _ = fmt.Fprint(w, `{"status":401,"detail":"invalid credentials","code":"care.invalid_credentials"}`)
			return
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"access_token":"token-%d"}}`, logins[r.URL.Path])
	})
	defer srv.Close()

	adapter := newLoginCachingAdapter(newSeedTestAdapter(srv.URL))
	ctx := t.Context()

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			auth, err := adapter.LoginTenant(ctx, "staff@example.test", "secret", "")
			assert.NoError(t, err)
			assert.Equal(t, "token-1", auth.Token)
		})
	}
	wg.Wait()

	other, err := adapter.LoginTenant(ctx, "staff@example.test", "secret", "demo")
	require.NoError(t, err)
	assert.Equal(t, "token-2", other.Token, "a different tenant slug is a separate login")

	_, err = adapter.LoginParent(ctx, "parent@example.test", "secret")
	require.Error(t, err)
	parent, err := adapter.LoginParent(ctx, "parent@example.test", "secret")
	require.NoError(t, err, "a failed login is not cached")
	assert.Equal(t, "token-2", parent.Token)

	assert.Equal(t, map[string]int{"/auth/login": 2, "/parent/auth/login": 2}, logins)
}

func TestNewLoginCachingAdapterKeepsNil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, newLoginCachingAdapter(nil))
}

func TestLoginCacheDoesNotReuseRetiredTenantSwitchFamily(t *testing.T) {
	t.Parallel()
	var logins atomic.Int64
	var refuseSwitch atomic.Bool
	refuseSwitch.Store(true)
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth/login" {
			_, _ = fmt.Fprintf(w, `{"access_token":"synthetic-session-%d"}`, logins.Add(1))
			return
		}
		if r.URL.Path == "/auth/switch-tenant" && refuseSwitch.Load() {
			w.WriteHeader(401)
			_, _ = fmt.Fprint(w, `{"status":401,"code":"identity.tenant_access_denied"}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"access_token":"synthetic-switched-session"}`)
	})
	defer srv.Close()
	adapter := newLoginCachingAdapter(newSeedTestAdapter(srv.URL))
	ctx := t.Context()
	source, err := adapter.LoginTenant(ctx, "shared@example.test", "example-test-password", "source")
	require.NoError(t, err)
	other, err := adapter.LoginTenant(ctx, "other@example.test", "example-test-password", "source")
	require.NoError(t, err)
	_, _, err = adapter.Raw(ctx, source, seedHTTPMethodPost, "/auth/switch-tenant", map[string]string{"tenant_slug": "target"}, nil)
	require.Error(t, err)
	cached, err := adapter.LoginTenant(ctx, "shared@example.test", "example-test-password", "source")
	require.NoError(t, err)
	require.Equal(t, source, cached, "a rejected switch does not retire its source family")
	refuseSwitch.Store(false)
	_, _, err = adapter.Raw(ctx, source, seedHTTPMethodPost, "/auth/switch-tenant", map[string]string{"tenant_slug": "target"}, nil)
	require.NoError(t, err)
	fresh, err := adapter.LoginTenant(ctx, "shared@example.test", "example-test-password", "source")
	require.NoError(t, err)
	require.NotEqual(t, source.Token, fresh.Token, "the source family is retired; re-enter the regular login/MFA flow")
	cachedOther, err := adapter.LoginTenant(ctx, "other@example.test", "example-test-password", "source")
	require.NoError(t, err)
	require.Equal(t, other, cachedOther, "other accounts must retain their cache entries")
}

func TestLoginCacheRetainsSwitchedSessionWithoutAnotherMFAExchange(t *testing.T) {
	t.Parallel()
	var logins, switches atomic.Int64
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth/login" {
			logins.Add(1)
			_, _ = fmt.Fprint(w, `{"access_token":"synthetic-login-session"}`)
			return
		}
		_, _ = fmt.Fprintf(w, `{"access_token":"synthetic-switch-%d"}`, switches.Add(1))
	})
	defer srv.Close()
	adapter := newLoginCachingAdapter(newSeedTestAdapter(srv.URL))
	ctx := t.Context()
	source, err := adapter.LoginTenant(ctx, "shared@example.test", "example-test-password", "source")
	require.NoError(t, err)
	_, _, err = adapter.Raw(ctx, source, seedHTTPMethodPost, "/auth/switch-tenant", map[string]string{"tenant_slug": "target"}, nil)
	require.NoError(t, err)
	target, err := adapter.LoginTenant(ctx, "shared@example.test", "example-test-password", "target")
	require.NoError(t, err)
	require.Equal(t, "synthetic-switch-1", target.Token)
	target.Label = "display-only label"
	_, _, err = adapter.Raw(ctx, target, seedHTTPMethodPost, "/auth/switch-tenant", map[string]any{"tenant_slug": "source"}, nil)
	require.NoError(t, err)
	returned, err := adapter.LoginTenant(ctx, "shared@example.test", "example-test-password", "source")
	require.NoError(t, err)
	require.Equal(t, "synthetic-switch-2", returned.Token)
	require.EqualValues(t, 1, logins.Load(), "reuse the regularly MFA-authenticated switched session, not another email challenge")
}
