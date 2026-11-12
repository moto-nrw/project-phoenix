package account_test

import "context"

// settingsDouble is a func-field double for the settings ports this
// package's code under test declares. A nil field answers the zero value.
type settingsDouble struct {
	ResolveStringForTenantFn func(ctx context.Context, tenantID int64, key string) (string, error)
	ResolveBoolForTenantFn   func(ctx context.Context, tenantID int64, key string) (bool, error)
	ResolveIntForTenantFn    func(ctx context.Context, tenantID int64, key string) (int, error)
}

func (s *settingsDouble) ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error) {
	if s.ResolveStringForTenantFn != nil {
		return s.ResolveStringForTenantFn(ctx, tenantID, key)
	}
	return "", nil
}

func (s *settingsDouble) ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error) {
	if s.ResolveBoolForTenantFn != nil {
		return s.ResolveBoolForTenantFn(ctx, tenantID, key)
	}
	return false, nil
}

func (s *settingsDouble) ResolveIntForTenant(ctx context.Context, tenantID int64, key string) (int, error) {
	if s.ResolveIntForTenantFn != nil {
		return s.ResolveIntForTenantFn(ctx, tenantID, key)
	}
	return 0, nil
}
