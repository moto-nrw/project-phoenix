package legacy

import (
	"context"
	"fmt"
	"strconv"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

type pickupChangePolicy struct {
	enabled bool
	cutoff  scheduleService.SameDayCutoff
}

// pickupChangeCutoffInTx resolves the cutoff at the guardian write boundary.
// It deliberately bypasses the request cache so a cutoff changed while this
// request was waiting on its row locks takes effect for the pending write.
func (s *service) pickupChangeCutoffInTx(ctx context.Context, tenantID int64) (scheduleService.SameDayCutoff, error) {
	policy, err := s.pickupChangePolicyInTx(ctx, tenantID)
	if err != nil {
		return scheduleService.SameDayCutoff{}, err
	}
	return policy.cutoff, nil
}

// pickupChangePolicyInTx re-reads the effective parent pickup-change policy
// after taking its shared lock. The corresponding settings writers hold the
// exclusive side until commit, so the enabled flag and cutoff describe one
// committed policy for the duration of the guardian write.
func (s *service) pickupChangePolicyInTx(ctx context.Context, tenantID int64) (pickupChangePolicy, error) {
	if err := s.Settings.LockParentPickupChangePolicySharedForTenant(ctx, tenantID); err != nil {
		return pickupChangePolicy{}, fmt.Errorf("parent: lock pickup-change policy: %w", err)
	}
	enabled, err := s.Settings.ResolveStringForTenantInTx(ctx, tenantID, configModels.KeyParentPickupChangeEnabled)
	if err != nil {
		return pickupChangePolicy{}, fmt.Errorf("parent: resolve pickup-change setting: %w", err)
	}
	pickupChangesEnabled, err := strconv.ParseBool(enabled)
	if err != nil {
		return pickupChangePolicy{}, fmt.Errorf("parent: parse pickup-change setting: %w", err)
	}
	if !pickupChangesEnabled {
		return pickupChangePolicy{}, nil
	}
	clock, err := s.Settings.ResolveStringForTenantInTx(ctx, tenantID, configModels.KeyParentPickupChangeCutoffTime)
	if err != nil {
		return pickupChangePolicy{}, fmt.Errorf("parent: resolve pickup-change cutoff: %w", err)
	}
	cutoff, err := scheduleService.NewSameDayCutoffAt(clock, s.now)
	if err != nil {
		return pickupChangePolicy{}, fmt.Errorf("parent: %w", err)
	}
	return pickupChangePolicy{enabled: true, cutoff: cutoff}, nil
}
