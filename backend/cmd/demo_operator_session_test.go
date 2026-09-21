package cmd

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingLogin stands in for the operator login with its second factor.
type countingLogin struct {
	mu     sync.Mutex
	logins int
	err    error
}

func (c *countingLogin) fetch() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logins++
	return "operator", c.err
}

func newTestSharedLogin(now *time.Time) *sharedLogin[string] {
	return &sharedLogin[string]{now: func() time.Time { return *now }}
}

// Parallel seed workers must not each drive the operator's second factor.
func TestSharedLoginSignsInOncePerLifetime(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	inner, session := &countingLogin{}, newTestSharedLogin(&now)

	var workers sync.WaitGroup
	for range demoSeedWorkers {
		workers.Go(func() {
			token, err := session.login(inner.fetch)
			assert.NoError(t, err)
			assert.Equal(t, "operator", token)
		})
	}
	workers.Wait()
	assert.Equal(t, 1, inner.logins)

	session.forget()
	_, err := session.login(inner.fetch)
	require.NoError(t, err)
	assert.Equal(t, 2, inner.logins, "a failed seed makes the repetition sign in again")

	now = now.Add(demoOperatorSessionLifetime)
	_, err = session.login(inner.fetch)
	require.NoError(t, err)
	assert.Equal(t, 3, inner.logins, "an old session is not handed to a new seed")
}

// A refused login (the second factor allows three codes in 15 minutes) must
// not be repeated by every waiting order, and it is never kept as a session.
func TestSharedLoginPausesAfterARefusedLogin(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	inner, session := &countingLogin{err: errors.New("429 - Too many code requests")}, newTestSharedLogin(&now)

	for range 5 {
		_, err := session.login(inner.fetch)
		require.Error(t, err)
	}
	assert.Equal(t, 1, inner.logins)
	assert.True(t, session.paused())

	now = now.Add(demoOperatorLoginPause)
	inner.err = nil
	assert.False(t, session.paused())
	token, err := session.login(inner.fetch)
	require.NoError(t, err)
	assert.Equal(t, "operator", token)
	assert.Equal(t, 2, inner.logins)
}
