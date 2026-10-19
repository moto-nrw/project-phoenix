package pwa

import "context"

// settingsDouble is a func-field double for the settings ports this
// package's code under test declares. A nil field answers the zero value.
type settingsDouble struct {
	ResolveIntFn func(ctx context.Context, key string) (int, error)
}

func (s *settingsDouble) ResolveInt(ctx context.Context, key string) (int, error) {
	if s.ResolveIntFn != nil {
		return s.ResolveIntFn(ctx, key)
	}
	return 0, nil
}
