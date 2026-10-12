// Package configtest provides a shared func-field double for the narrow
// settings ports consumers declare (ResolveBool, ResolveString, ...). It
// carries only primitive-typed resolvers and imports no settings package, so
// it satisfies each consumer's own port without reaching the Settings
// Platform's implementation (#3965). A consumer that needs schema or snapshot
// types declares a local fake next to its test.
//
// Each method delegates to its Fn field; a nil field returns zero values.
// Tests configure only the methods they exercise:
//
//	settings := &configtest.Mock{
//		ResolveBoolFn: func(ctx context.Context, key string) (bool, error) { return true, nil },
//	}
package configtest

import "context"

// Mock is a func-field test double for consumer-owned settings ports.
type Mock struct {
	ResolveStringFn          func(ctx context.Context, key string) (string, error)
	ResolveStringForTenantFn func(ctx context.Context, tenantID int64, key string) (string, error)
	ResolveBoolFn            func(ctx context.Context, key string) (bool, error)
	ResolveBoolsFn           func(ctx context.Context, keys []string) (map[string]bool, error)
	ResolveBoolForTenantFn   func(ctx context.Context, tenantID int64, key string) (bool, error)
	ResolveIntFn             func(ctx context.Context, key string) (int, error)
	ResolveIntForTenantFn    func(ctx context.Context, tenantID int64, key string) (int, error)
	HasTenantOverrideFn      func(ctx context.Context, key string) (bool, error)
	GetLoginImageURLFn       func(ctx context.Context, tenantID int64) (string, error)
}

func (m *Mock) ResolveString(ctx context.Context, key string) (string, error) {
	if m.ResolveStringFn != nil {
		return m.ResolveStringFn(ctx, key)
	}
	return "", nil
}

func (m *Mock) ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error) {
	if m.ResolveStringForTenantFn != nil {
		return m.ResolveStringForTenantFn(ctx, tenantID, key)
	}
	return "", nil
}

func (m *Mock) ResolveBool(ctx context.Context, key string) (bool, error) {
	if m.ResolveBoolFn != nil {
		return m.ResolveBoolFn(ctx, key)
	}
	return false, nil
}

func (m *Mock) ResolveBools(ctx context.Context, keys []string) (map[string]bool, error) {
	if m.ResolveBoolsFn != nil {
		return m.ResolveBoolsFn(ctx, keys)
	}
	resolved := make(map[string]bool, len(keys))
	for _, key := range keys {
		value, err := m.ResolveBool(ctx, key)
		if err != nil {
			return nil, err
		}
		resolved[key] = value
	}
	return resolved, nil
}

func (m *Mock) ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error) {
	if m.ResolveBoolForTenantFn != nil {
		return m.ResolveBoolForTenantFn(ctx, tenantID, key)
	}
	return false, nil
}

func (m *Mock) ResolveInt(ctx context.Context, key string) (int, error) {
	if m.ResolveIntFn != nil {
		return m.ResolveIntFn(ctx, key)
	}
	return 0, nil
}

func (m *Mock) ResolveIntForTenant(ctx context.Context, tenantID int64, key string) (int, error) {
	if m.ResolveIntForTenantFn != nil {
		return m.ResolveIntForTenantFn(ctx, tenantID, key)
	}
	return 0, nil
}

func (m *Mock) HasTenantOverride(ctx context.Context, key string) (bool, error) {
	if m.HasTenantOverrideFn != nil {
		return m.HasTenantOverrideFn(ctx, key)
	}
	return false, nil
}

func (m *Mock) GetLoginImageURL(ctx context.Context, tenantID int64) (string, error) {
	if m.GetLoginImageURLFn != nil {
		return m.GetLoginImageURLFn(ctx, tenantID)
	}
	return "", nil
}
