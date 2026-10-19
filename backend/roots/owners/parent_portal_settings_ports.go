package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// BindParentSettings preserves optional batching while translating the
// Settings owner's snapshot to the guardian workflow's read port.
func BindParentSettings(source parentportal.SettingsService) parentportal.SettingsService {
	if batch, ok := source.(parentSettingsBatchSource); ok {
		return ParentSettingsBatch{Source: batch}
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

type ParentSettingsBatch struct{ Source parentSettingsBatchSource }

func (s ParentSettingsBatch) ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error) {
	return s.Source.ResolveBoolForTenant(ctx, tenantID, key)
}

func (s ParentSettingsBatch) ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error) {
	return s.Source.ResolveStringForTenant(ctx, tenantID, key)
}

func (s ParentSettingsBatch) ResolveStringForTenantInTx(ctx context.Context, tenantID int64, key string) (string, error) {
	return s.Source.ResolveStringForTenantInTx(ctx, tenantID, key)
}

func (s ParentSettingsBatch) LockParentPickupChangePolicySharedForTenant(ctx context.Context, tenantID int64) error {
	return s.Source.LockParentPickupChangePolicySharedForTenant(ctx, tenantID)
}

func (s ParentSettingsBatch) ResolveManyForTenant(ctx context.Context, tenantID int64, keys []string) (parentportal.SettingsSnapshot, error) {
	snapshot, err := s.Source.ResolveManyForTenant(ctx, tenantID, keys)
	if snapshot == nil {
		return nil, err
	}
	return snapshot, err
}
