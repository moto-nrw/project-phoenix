package legacy

import (
	"context"

	settingsplatform "github.com/moto-nrw/project-phoenix/modules/settings"
)

// settingsDouble is a func-field double for the settings ports this
// package's code under test declares. A nil field answers the zero value.
type settingsDouble struct {
	ResolveStringFn func(ctx context.Context, key string) (string, error)
	ResolveBoolFn   func(ctx context.Context, key string) (bool, error)
	ResolveManyFn   func(ctx context.Context, keys []string) (*settingsplatform.Snapshot, error)
}

func (s *settingsDouble) ResolveString(ctx context.Context, key string) (string, error) {
	if s.ResolveStringFn != nil {
		return s.ResolveStringFn(ctx, key)
	}
	return "", nil
}

func (s *settingsDouble) ResolveBool(ctx context.Context, key string) (bool, error) {
	if s.ResolveBoolFn != nil {
		return s.ResolveBoolFn(ctx, key)
	}
	return false, nil
}

// ResolveMany answers no snapshot by default, so Prepare keeps the request
// context unchanged.
func (s *settingsDouble) ResolveMany(ctx context.Context, keys []string) (*settingsplatform.Snapshot, error) {
	if s.ResolveManyFn != nil {
		return s.ResolveManyFn(ctx, keys)
	}
	return nil, nil
}
