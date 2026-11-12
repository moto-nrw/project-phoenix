package config

import (
	"context"
	"fmt"

	config "github.com/moto-ogs/project-phoenix/modules/settings/records"
)

func (s *settingsService) TimetableEnabled(ctx context.Context) (bool, error) {
	return s.ResolveBool(ctx, config.KeyTimetableEnabled)
}

// PickupCutoffs preserves the writer's existing shared-lock barrier. A batch
// is safe for fresh reads because both school-only keys share that lock.
// Explicit snapshots retain the scalar path, including partial snapshots.
func (s *settingsService) PickupCutoffs(ctx context.Context) (string, string, error) {
	if err := s.LockSlotListCutoffPairShared(ctx); err != nil {
		return "", "", err
	}
	if !s.canBatchSlotListCutoffs(ctx) {
		return s.scalarCutoffs(ctx)
	}
	values, err := s.ResolveMany(ctx, []string{config.KeySlotListShortDayCutoff, config.KeySlotListLongDayCutoff})
	if err != nil {
		return "", "", fmt.Errorf("resolve short-day pickup cutoff: %w", err)
	}
	short, err := values.String(config.KeySlotListShortDayCutoff)
	if err != nil {
		return "", "", fmt.Errorf("resolve short-day pickup cutoff: %w", err)
	}
	long, err := values.String(config.KeySlotListLongDayCutoff)
	if err != nil {
		return "", "", fmt.Errorf("resolve long-day pickup cutoff: %w", err)
	}
	return short, long, nil
}

func (s *settingsService) scalarCutoffs(ctx context.Context) (string, string, error) {
	short, err := s.ResolveString(ctx, config.KeySlotListShortDayCutoff)
	if err != nil {
		return "", "", fmt.Errorf("resolve short-day pickup cutoff: %w", err)
	}
	long, err := s.ResolveString(ctx, config.KeySlotListLongDayCutoff)
	if err != nil {
		return "", "", fmt.Errorf("resolve long-day pickup cutoff: %w", err)
	}
	return short, long, nil
}

// Preserve scalar semantics when there is no held transaction lock, an
// explicit snapshot, or a special registry/cache context. The usual HTTP
// path has two registered school-only keys and an empty post-lock memo.
func (s *settingsService) canBatchSlotListCutoffs(ctx context.Context) bool {
	if s.runtime == nil || !s.runtime.HasTransaction(ctx) || config.HasExplicitSettingsSnapshot(ctx) {
		return false
	}
	keys := []string{config.KeySlotListShortDayCutoff, config.KeySlotListLongDayCutoff}
	for _, key := range keys {
		def := s.registry.GetDefinition(key)
		if def == nil || def.Levels.AtTraeger() {
			return false
		}
	}
	if cache := config.RequestCacheFromContext(ctx); cache != nil {
		_, missing := cache.Lookup(s.tenantID(ctx), keys)
		return len(missing) == len(keys)
	}
	return true
}
