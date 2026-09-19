package api

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			_, _ = fmt.Fprint(w, `{"status":"error","message":"invalid credentials"}`)
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
