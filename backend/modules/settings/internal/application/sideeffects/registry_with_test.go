package sideeffects

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistryWithBindsHandlersAndObserverAtConstruction(t *testing.T) {
	t.Parallel()
	var failed []string
	errHandler := errors.New("handler failed")
	registry, err := NewRegistryWith(func(key string) { failed = append(failed, key) },
		Handler{Key: "operations.ok", Handle: func(context.Context, int64, any) (func(), error) { return nil, nil }},
		Handler{Key: "operations.fails", Handle: func(context.Context, int64, any) (func(), error) { return nil, errHandler }},
	)
	require.NoError(t, err)

	_, err = registry.Dispatch(context.Background(), 7, "operations.ok", true)
	require.NoError(t, err)
	_, err = registry.Dispatch(context.Background(), 7, "operations.fails", true)
	require.ErrorIs(t, err, errHandler)
	assert.Equal(t, []string{"operations.fails"}, failed)
}

func TestNewRegistryWithRejectsAnIncompleteRegistry(t *testing.T) {
	t.Parallel()
	handle := func(context.Context, int64, any) (func(), error) { return nil, nil }
	observe := func(string) {}

	_, err := NewRegistryWith(nil, Handler{Key: "k", Handle: handle})
	require.EqualError(t, err, "sideeffects: failure observer is required")
	_, err = NewRegistryWith(observe, Handler{Key: "k"})
	require.EqualError(t, err, `sideeffects: nil handler for key "k"`)
	_, err = NewRegistryWith(observe, Handler{Key: "k", Handle: handle}, Handler{Key: "k", Handle: handle})
	require.EqualError(t, err, `sideeffects: handler already registered for key "k"`)
}
