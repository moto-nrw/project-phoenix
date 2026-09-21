package cmd

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingOperatorAdapter struct {
	seedapi.Adapter
	mu     sync.Mutex
	logins int
	err    error
}

func (a *countingOperatorAdapter) LoginOperator(context.Context, string, string) (seedapi.AuthRef, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.logins++
	return seedapi.AuthRef{Kind: seedapi.AuthBearer, Token: "operator"}, a.err
}

// Parallel seed workers must not each drive the operator's second factor.
func TestSharedOperatorSessionSignsInOncePerLifetime(t *testing.T) {
	t.Parallel()
	inner := &countingOperatorAdapter{}
	session := newSharedOperatorSession(inner)
	now := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	session.now = func() time.Time { return now }

	var workers sync.WaitGroup
	for range demoSeedWorkers {
		workers.Go(func() {
			auth, err := session.LoginOperator(context.Background(), "operator@example.test", "secret")
			assert.NoError(t, err)
			assert.Equal(t, "operator", auth.Token)
		})
	}
	workers.Wait()
	assert.Equal(t, 1, inner.logins)

	now = now.Add(demoOperatorSessionLifetime)
	_, err := session.LoginOperator(context.Background(), "operator@example.test", "secret")
	require.NoError(t, err)
	assert.Equal(t, 2, inner.logins, "an old session is not handed to a new seed")
}

// A refused login (the second factor allows three codes in 15 minutes) must
// not be repeated by every waiting order, and it is never kept as a session.
func TestSharedOperatorSessionPausesAfterARefusedLogin(t *testing.T) {
	t.Parallel()
	inner := &countingOperatorAdapter{err: errors.New("429 - Too many code requests")}
	session := newSharedOperatorSession(inner)
	now := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	session.now = func() time.Time { return now }

	for range 5 {
		_, err := session.LoginOperator(context.Background(), "operator@example.test", "secret")
		require.Error(t, err)
	}
	assert.Equal(t, 1, inner.logins)
	assert.True(t, session.paused())

	now = now.Add(demoOperatorLoginPause)
	inner.err = nil
	assert.False(t, session.paused())
	_, err := session.LoginOperator(context.Background(), "operator@example.test", "secret")
	require.NoError(t, err)
	assert.Equal(t, 2, inner.logins)
}
