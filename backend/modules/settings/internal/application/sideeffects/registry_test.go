package sideeffects

import (
	"context"
	"errors"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegistry_Dispatch_RoutesToRegisteredHandler pins the core contract:
// Dispatch finds the handler keyed to the setting key, calls it with the
// dispatch arguments, and returns the handler's (postCommit, err).
func TestRegistry_Dispatch_RoutesToRegisteredHandler(t *testing.T) {
	t.Parallel()

	var capturedKey string
	var capturedTenantID int64
	var capturedValue any
	r, err := NewRegistryWith(func(string) {}, Handler{Key: "operations.foo", Handle: func(_ context.Context, tenantID int64, value any) (func(), error) {
		capturedTenantID = tenantID
		capturedKey = "operations.foo"
		capturedValue = value
		return nil, nil
	}})
	require.NoError(t, err)

	cb, err := r.Dispatch(context.Background(), int64(42), "operations.foo", true)
	require.NoError(t, err)
	assert.Nil(t, cb, "handler returned nil postCommit — Dispatch must propagate it")
	assert.Equal(t, int64(42), capturedTenantID)
	assert.Equal(t, "operations.foo", capturedKey)
	assert.Equal(t, true, capturedValue)
}

// TestRegistry_Dispatch_UnknownKeyIsNoop ensures an unregistered key does
// NOT explode — the API dispatches every settings write through the
// registry, and most keys legitimately have no handler.
func TestRegistry_Dispatch_UnknownKeyIsNoop(t *testing.T) {
	t.Parallel()

	r, err := NewRegistryWith(func(string) {})
	require.NoError(t, err)

	cb, err := r.Dispatch(context.Background(), testpkg.UniqueTestTenantID(t), "unknown.key", "any")
	assert.NoError(t, err)
	assert.Nil(t, cb)
}

// TestRegistry_Dispatch_PropagatesPostCommit confirms a handler that
// returns a non-nil postCommit closure has it surfaced unchanged — the
// settings handlers schedule it via tenant.RegisterAfterCommit.
func TestRegistry_Dispatch_PropagatesPostCommit(t *testing.T) {
	t.Parallel()

	postCommitRan := false
	r, err := NewRegistryWith(func(string) {}, Handler{Key: "k", Handle: func(_ context.Context, _ int64, _ any) (func(), error) {
		return func() { postCommitRan = true }, nil
	}})
	require.NoError(t, err)

	cb, err := r.Dispatch(context.Background(), 1, "k", nil)
	require.NoError(t, err)
	require.NotNil(t, cb)
	cb()
	assert.True(t, postCommitRan)
}

// TestRegistry_Dispatch_PropagatesError verifies an in-tx handler error
// returns through Dispatch unwrapped so the outer SettingsResource can
// roll back the tx via its err return.
func TestRegistry_Dispatch_PropagatesError(t *testing.T) {
	t.Parallel()

	want := errors.New("boom")
	var observed string
	r, err := NewRegistryWith(func(key string) { observed = key }, Handler{Key: "k", Handle: func(_ context.Context, _ int64, _ any) (func(), error) {
		return nil, want
	}})
	require.NoError(t, err)

	cb, err := r.Dispatch(context.Background(), 1, "k", nil)
	assert.Same(t, want, err)
	assert.Nil(t, cb)
	assert.Equal(t, "k", observed)
}

// TestRegistry_Construction_RefusesDuplicateKey locks the boot-time check
// that catches double-registration mistakes during refactors. Two
// handlers for the same key would silently shadow each other.
func TestRegistry_Construction_RefusesDuplicateKey(t *testing.T) {
	t.Parallel()

	handle := func(_ context.Context, _ int64, _ any) (func(), error) { return nil, nil }
	_, err := NewRegistryWith(func(string) {}, Handler{Key: "k", Handle: handle}, Handler{Key: "k", Handle: handle})
	assert.EqualError(t, err, `sideeffects: handler already registered for key "k"`)
}

// TestRegistry_Construction_RefusesNilHandler catches the obvious mistake of
// passing nil — at boot time, not in production when Dispatch would
// otherwise crash.
func TestRegistry_Construction_RefusesNilHandler(t *testing.T) {
	t.Parallel()

	_, err := NewRegistryWith(func(string) {}, Handler{Key: "k"})
	assert.EqualError(t, err, `sideeffects: nil handler for key "k"`)
}
