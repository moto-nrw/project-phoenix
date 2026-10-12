package announcement_test

import "context"

// settingsDouble is a func-field double for the settings ports this
// package's code under test declares. A nil field answers the zero value.
type settingsDouble struct {
	ResolveStringForTenantFn func(ctx context.Context, tenantID int64, key string) (string, error)
	ResolveBoolFn            func(ctx context.Context, key string) (bool, error)
	ResolveBoolForTenantFn   func(ctx context.Context, tenantID int64, key string) (bool, error)
	GetLoginImageURLFn       func(ctx context.Context, tenantID int64) (string, error)
}

func (s *settingsDouble) ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error) {
	if s.ResolveStringForTenantFn != nil {
		return s.ResolveStringForTenantFn(ctx, tenantID, key)
	}
	return "", nil
}

func (s *settingsDouble) ResolveBool(ctx context.Context, key string) (bool, error) {
	if s.ResolveBoolFn != nil {
		return s.ResolveBoolFn(ctx, key)
	}
	return false, nil
}

func (s *settingsDouble) ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error) {
	if s.ResolveBoolForTenantFn != nil {
		return s.ResolveBoolForTenantFn(ctx, tenantID, key)
	}
	return false, nil
}

func (s *settingsDouble) GetLoginImageURL(ctx context.Context, tenantID int64) (string, error) {
	if s.GetLoginImageURLFn != nil {
		return s.GetLoginImageURLFn(ctx, tenantID)
	}
	return "", nil
}
