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
	ResolveManyFn   func(ctx context.Context, keys []string) (*settingsplatform.Snapshot, error)
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

func (s *settingsDouble) ResolveMany(ctx context.Context, keys []string) (*settingsplatform.Snapshot, error) {
	if s.ResolveManyFn != nil {
		return s.ResolveManyFn(ctx, keys)
	}
	return nil, nil
}

func TestSettingsPrepareFetchesOneSnapshot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var requested []string
	prepared, err := settings{settings: &settingsDouble{ResolveManyFn: func(_ context.Context, keys []string) (*settingsplatform.Snapshot, error) {
		requested = keys
		return &settingsplatform.Snapshot{}, nil
	}}}.Prepare(ctx)
	require.NoError(t, err)
	assert.NotEqual(t, ctx, prepared, "the snapshot travels in the returned context")
	assert.Contains(t, requested, settingsplatform.KeySessionEndTime)
	assert.Contains(t, requested, settingsplatform.KeyPresenceMode)

	_, err = settings{settings: &settingsDouble{ResolveManyFn: func(context.Context, []string) (*settingsplatform.Snapshot, error) {
		return nil, errors.New("settings down")
	}}}.Prepare(ctx)
	require.EqualError(t, err, "settings down")

	// No snapshot keeps the request context unchanged.
	unprepared, err := settings{settings: &settingsDouble{}}.Prepare(ctx)
	require.NoError(t, err)
	assert.Equal(t, ctx, unprepared)
}
