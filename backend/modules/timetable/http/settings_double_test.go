package timetablehttp

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/settings"
)

// settingsDouble is a func-field double for the settings ports this
// package's code under test declares. A nil field answers the zero value.
type settingsDouble struct {
	ResolveStringFn     func(ctx context.Context, key string) (string, error)
	ResolveBoolFn       func(ctx context.Context, key string) (bool, error)
	ResolveIntFn        func(ctx context.Context, key string) (int, error)
	HasTenantOverrideFn func(ctx context.Context, key string) (bool, error)
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

func (s *settingsDouble) ResolveInt(ctx context.Context, key string) (int, error) {
	if s.ResolveIntFn != nil {
		return s.ResolveIntFn(ctx, key)
	}
	return 0, nil
}

func (s *settingsDouble) HasTenantOverride(ctx context.Context, key string) (bool, error) {
	if s.HasTenantOverrideFn != nil {
		return s.HasTenantOverrideFn(ctx, key)
	}
	return false, nil
}

// Route tests supply a deliberate settings double even when testing other ports.
func newResourceForTest(deps Dependencies) *Resource {
	if deps.SettingsService == nil {
		deps.SettingsService = &settingsDouble{
			ResolveBoolFn: func(context.Context, string) (bool, error) { return true, nil },
			ResolveIntFn: func(_ context.Context, key string) (int, error) {
				switch key {
				case settings.KeyEnrollmentGradeLevelMax:
					return 4, nil
				case settings.KeyTimetableChildrenPerStaffRatio:
					return 12, nil
				default:
					return 0, nil
				}
			},
		}
	}
	return NewResource(deps)
}
