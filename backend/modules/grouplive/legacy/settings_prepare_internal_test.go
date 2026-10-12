package legacy

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	settingsplatform "github.com/moto-nrw/project-phoenix/modules/settings"
)

// settingsDouble is a func-field double for the projection's settings port.
// A nil field answers the zero value.
type settingsDouble struct {
	ResolveBoolFn   func(ctx context.Context, key string) (bool, error)
	ResolveStringFn func(ctx context.Context, key string) (string, error)
	PrefetchFn      func(ctx context.Context, keys []string) (context.Context, error)
}

func (s *settingsDouble) ResolveBool(ctx context.Context, key string) (bool, error) {
	if s.ResolveBoolFn != nil {
		return s.ResolveBoolFn(ctx, key)
	}
	return false, nil
}

func (s *settingsDouble) ResolveString(ctx context.Context, key string) (string, error) {
	if s.ResolveStringFn != nil {
		return s.ResolveStringFn(ctx, key)
	}
	return "", nil
}

func (s *settingsDouble) Prefetch(ctx context.Context, keys []string) (context.Context, error) {
	if s.PrefetchFn != nil {
		return s.PrefetchFn(ctx, keys)
	}
	return ctx, nil
}

func TestSettingsPreparePrefetchesTheProjectionKeys(t *testing.T) {
	t.Parallel()

	type prefetched struct{}
	ctx := context.Background()
	var requested []string
	prepared, err := settings{settings: &settingsDouble{PrefetchFn: func(ctx context.Context, keys []string) (context.Context, error) {
		requested = keys
		return context.WithValue(ctx, prefetched{}, true), nil
	}}}.Prepare(ctx)
	require.NoError(t, err)
	assert.Equal(t, true, prepared.Value(prefetched{}), "the request continues with the prefetched context")
	assert.Contains(t, requested, settingsplatform.KeySessionEndTime)
	assert.Contains(t, requested, settingsplatform.KeyPresenceMode)

	_, err = settings{settings: &settingsDouble{PrefetchFn: func(ctx context.Context, _ []string) (context.Context, error) {
		return ctx, errors.New("settings down")
	}}}.Prepare(ctx)
	require.EqualError(t, err, "settings down")
}
