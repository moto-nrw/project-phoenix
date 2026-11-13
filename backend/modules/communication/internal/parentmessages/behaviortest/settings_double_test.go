package messaging_test

import "context"

// settingsDouble is a func-field double for the settings ports this
// package's code under test declares. A nil field answers the zero value.
type settingsDouble struct {
	ResolveBoolFn func(ctx context.Context, key string) (bool, error)
	ResolveIntFn  func(ctx context.Context, key string) (int, error)
}

func (s *settingsDouble) ResolveBool(ctx context.Context, key string) (bool, error) {
	if s.ResolveBoolFn != nil {
		return s.ResolveBoolFn(ctx, key)
	}
	return false, nil
}

func (s *settingsDouble) ResolveInt(ctx context.Context, key string) (int, error) {
	if s.ResolveIntFn != nil {
		return s.ResolveIntFn(ctx, key)
	}
	return 0, nil
}
