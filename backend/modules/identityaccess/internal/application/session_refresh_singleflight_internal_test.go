package application

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type blockedRefreshCodec struct {
	fakeCodec
	release chan struct{}
	parses  atomic.Int32
}

func (c *blockedRefreshCodec) ParseRefreshToken(token string) (domain.RefreshClaims, error) {
	c.parses.Add(1)
	<-c.release
	return c.fakeCodec.ParseRefreshToken(token)
}

// Concurrent refreshes must share one rotation and the exact same token pair.
// A start barrier alone cannot guarantee overlap: a late caller without a
// recovery proof correctly rejects the already-rotated token as a replay.
func TestAuthService_RefreshToken_ConcurrentSingleflight(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		f := newAuthFixture(t)
		f.seedStaff(1, 10, "user")
		ctx := context.Background()
		_, refreshToken, err := f.auth.loginWithAudit(ctx, "user1@example.com", "secret", "", "", "")
		require.NoError(t, err)

		codec := &blockedRefreshCodec{release: make(chan struct{})}
		f.auth.codec = codec
		const concurrency = 5
		type result struct {
			accessToken  string
			refreshToken string
			err          error
		}
		results := make(chan result, concurrency)
		for range concurrency {
			go func() {
				at, rt, e := f.auth.refreshTokenWithAudit(ctx, refreshToken, "", "")
				results <- result{at, rt, e}
			}()
		}

		// The leader is blocked in parsing and the other callers in singleflight.
		// Wait establishes overlap without sleeps or scheduler assumptions.
		synctest.Wait()
		assert.Equal(t, int32(1), codec.parses.Load(), "only the leader should parse the refresh token")
		close(codec.release)

		var successes int
		var firstAccess, firstRefresh string
		for range concurrency {
			r := <-results
			if r.err != nil {
				t.Errorf("concurrent refresh failed: %v", r.err)
				continue
			}
			successes++
			assert.NotEmpty(t, r.accessToken, "access token should not be empty")
			assert.NotEmpty(t, r.refreshToken, "refresh token should not be empty")

			if firstAccess == "" {
				firstAccess = r.accessToken
				firstRefresh = r.refreshToken
			} else {
				assert.Equal(t, firstAccess, r.accessToken, "all callers should get same access token")
				assert.Equal(t, firstRefresh, r.refreshToken, "all callers should get same refresh token")
			}
		}
		assert.Equal(t, concurrency, successes, "all concurrent refresh calls should succeed")
	})
}
