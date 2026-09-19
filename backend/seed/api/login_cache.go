package api

import (
	"context"
	"sync"
)

// loginCachingAdapter reuses the access token of an earlier tenant or parent
// login with the same credentials. Every login pays a deliberate Argon2 cost
// on the server, and the seeder signs the same accounts in again and again
// across steps. One seed run stays far inside the token lifetime, and the
// seeder never changes roles or permissions of an account that already logged
// in, so the claims of a reused token stay current. Failed logins are not
// cached. Operator logins pass through because they drive the MFA flow.
type loginCachingAdapter struct {
	Adapter
	mu     sync.Mutex
	logins map[loginCacheKey]*cachedLogin
}

type cachedLogin struct {
	mu   sync.Mutex
	auth AuthRef
	done bool
}

type loginCacheKey struct {
	portal, email, password, tenantSlug string
}

func newLoginCachingAdapter(inner Adapter) Adapter {
	if inner == nil {
		return nil
	}
	return &loginCachingAdapter{Adapter: inner, logins: make(map[loginCacheKey]*cachedLogin)}
}

func (a *loginCachingAdapter) LoginTenant(ctx context.Context, email, password, tenantSlug string) (AuthRef, error) {
	return a.login(loginCacheKey{"tenant", email, password, tenantSlug}, func() (AuthRef, error) {
		return a.Adapter.LoginTenant(ctx, email, password, tenantSlug)
	})
}

func (a *loginCachingAdapter) LoginParent(ctx context.Context, email, password string) (AuthRef, error) {
	return a.login(loginCacheKey{"parent", email, password, ""}, func() (AuthRef, error) {
		return a.Adapter.LoginParent(ctx, email, password)
	})
}

// login locks per account, so concurrent workers signing in as the same
// account wait for one login while logins of other accounts run in parallel.
func (a *loginCachingAdapter) login(key loginCacheKey, fetch func() (AuthRef, error)) (AuthRef, error) {
	a.mu.Lock()
	entry, ok := a.logins[key]
	if !ok {
		entry = &cachedLogin{}
		a.logins[key] = entry
	}
	a.mu.Unlock()

	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.done {
		return entry.auth, nil
	}
	auth, err := fetch()
	if err != nil {
		return auth, err
	}
	entry.auth, entry.done = auth, true
	return auth, nil
}
