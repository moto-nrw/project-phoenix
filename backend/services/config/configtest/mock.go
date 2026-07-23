// Package configtest provides a shared func-field mock for the
// config.SettingsService interface, replacing the per-package full-interface
// stubs that used to be copy-pasted across api and services test files.
//
// Each method delegates to its Fn field; a nil field returns zero values.
// Tests configure only the methods they exercise:
//
//	settings := &configtest.Mock{
//		ResolveBoolFn: func(ctx context.Context, key string) (bool, error) { return true, nil },
//	}
package configtest

import (
	"context"

	config "github.com/moto-nrw/project-phoenix/services/config"
)

// Mock is a func-field test double for config.SettingsService.
type Mock struct {
	GetSchemaFn              func(ctx context.Context, userPermissions []string) (*config.SettingsSchema, error)
	GetSchemaForOperatorFn   func(ctx context.Context, userPermissions []string) (*config.SettingsSchema, error)
	ResolveFn                func(ctx context.Context, key string) (any, error)
	ResolveStringFn          func(ctx context.Context, key string) (string, error)
	ResolveStringForTenantFn func(ctx context.Context, tenantID int64, key string) (string, error)
	ResolveBoolFn            func(ctx context.Context, key string) (bool, error)
	ResolveBoolForTenantFn   func(ctx context.Context, tenantID int64, key string) (bool, error)
	ResolveIntFn             func(ctx context.Context, key string) (int, error)
	ResolveIntForTenantFn    func(ctx context.Context, tenantID int64, key string) (int, error)
	HasTenantOverrideFn      func(ctx context.Context, key string) (bool, error)
	SetValueFn               func(ctx context.Context, key string, value any, changedBy *int64, userPermissions []string) error
	ResetValueFn             func(ctx context.Context, key string, changedBy *int64, userPermissions []string) error
	CheckOperatorWritableFn  func(key string) error
	GetLoginImageURLFn       func(ctx context.Context, tenantID int64) (string, error)
	SetLoginImageURLFn       func(ctx context.Context, tenantID int64, imageURL string) (string, error)
	ClearLoginImageURLFn     func(ctx context.Context, tenantID int64) (string, error)
	LockSlotListCutoffPairFn func(ctx context.Context) error
}

var _ config.SettingsService = (*Mock)(nil)

func (m *Mock) GetSchema(ctx context.Context, userPermissions []string) (*config.SettingsSchema, error) {
	if m.GetSchemaFn != nil {
		return m.GetSchemaFn(ctx, userPermissions)
	}
	return nil, nil
}

func (m *Mock) GetSchemaForOperator(ctx context.Context, userPermissions []string) (*config.SettingsSchema, error) {
	if m.GetSchemaForOperatorFn != nil {
		return m.GetSchemaForOperatorFn(ctx, userPermissions)
	}
	return nil, nil
}

func (m *Mock) Resolve(ctx context.Context, key string) (any, error) {
	if m.ResolveFn != nil {
		return m.ResolveFn(ctx, key)
	}
	return nil, nil
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

func (m *Mock) SetValue(ctx context.Context, key string, value any, changedBy *int64, userPermissions []string) error {
	if m.SetValueFn != nil {
		return m.SetValueFn(ctx, key, value, changedBy, userPermissions)
	}
	return nil
}

func (m *Mock) ResetValue(ctx context.Context, key string, changedBy *int64, userPermissions []string) error {
	if m.ResetValueFn != nil {
		return m.ResetValueFn(ctx, key, changedBy, userPermissions)
	}
	return nil
}

func (m *Mock) CheckOperatorWritable(key string) error {
	if m.CheckOperatorWritableFn != nil {
		return m.CheckOperatorWritableFn(key)
	}
	return nil
}

func (m *Mock) GetLoginImageURL(ctx context.Context, tenantID int64) (string, error) {
	if m.GetLoginImageURLFn != nil {
		return m.GetLoginImageURLFn(ctx, tenantID)
	}
	return "", nil
}

func (m *Mock) SetLoginImageURL(ctx context.Context, tenantID int64, imageURL string) (string, error) {
	if m.SetLoginImageURLFn != nil {
		return m.SetLoginImageURLFn(ctx, tenantID, imageURL)
	}
	return "", nil
}

func (m *Mock) ClearLoginImageURL(ctx context.Context, tenantID int64) (string, error) {
	if m.ClearLoginImageURLFn != nil {
		return m.ClearLoginImageURLFn(ctx, tenantID)
	}
	return "", nil
}

func (m *Mock) LockSlotListCutoffPair(ctx context.Context) error {
	if m.LockSlotListCutoffPairFn != nil {
		return m.LockSlotListCutoffPairFn(ctx)
	}
	return nil
}
