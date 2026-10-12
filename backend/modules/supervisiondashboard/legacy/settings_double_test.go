package legacy

import "context"

// settingsDouble is a func-field double for the settings ports this
// package's code under test declares. A nil field answers the zero value.
type settingsDouble struct {
	ResolveStringFn func(ctx context.Context, key string) (string, error)
	ResolveBoolFn   func(ctx context.Context, key string) (bool, error)
	PrefetchFn      func(ctx context.Context, keys []string) (context.Context, error)
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

// Prefetch keeps the request context by default.
func (s *settingsDouble) Prefetch(ctx context.Context, keys []string) (context.Context, error) {
	if s.PrefetchFn != nil {
		return s.PrefetchFn(ctx, keys)
	}
	return ctx, nil
}
