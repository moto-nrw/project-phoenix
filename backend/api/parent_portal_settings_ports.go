package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// BindParentSettings preserves optional batching while translating the
// Settings owner's snapshot to the guardian workflow's read port.
func BindParentSettings(source parentportal.SettingsService) parentportal.SettingsService {
	if batch, ok := source.(parentSettingsBatchSource); ok {
		return parentSettingsBatch{source: batch}
	}
	return source
}

type parentSettingsBatchSource interface {
	ResolveBoolForTenant(context.Context, int64, string) (bool, error)
	ResolveStringForTenant(context.Context, int64, string) (string, error)
	ResolveStringForTenantInTx(context.Context, int64, string) (string, error)
	LockParentPickupChangePolicySharedForTenant(context.Context, int64) error
	ResolveManyForTenant(context.Context, int64, []string) (*settings.Snapshot, error)
}

type parentSettingsBatch struct{ source parentSettingsBatchSource }

var _ parentportal.BatchSettingsService = parentSettingsBatch{}

func (s parentSettingsBatch) ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error) {
	return s.source.ResolveBoolForTenant(ctx, tenantID, key)
}

func (s parentSettingsBatch) ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error) {
	return s.source.ResolveStringForTenant(ctx, tenantID, key)
}

func (s parentSettingsBatch) ResolveStringForTenantInTx(ctx context.Context, tenantID int64, key string) (string, error) {
	return s.source.ResolveStringForTenantInTx(ctx, tenantID, key)
}

func (s parentSettingsBatch) LockParentPickupChangePolicySharedForTenant(ctx context.Context, tenantID int64) error {
	return s.source.LockParentPickupChangePolicySharedForTenant(ctx, tenantID)
}

func (s parentSettingsBatch) ResolveManyForTenant(ctx context.Context, tenantID int64, keys []string) (parentportal.SettingsSnapshot, error) {
	snapshot, err := s.source.ResolveManyForTenant(ctx, tenantID, keys)
	if snapshot == nil {
		return nil, err
	}
	return snapshot, err
}
