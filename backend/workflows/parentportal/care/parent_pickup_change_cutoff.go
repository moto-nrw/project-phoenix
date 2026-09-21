package care

import (
	"context"
	"fmt"
	"strconv"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type pickupChangePolicy struct {
	enabled bool
	cutoff  careplan.SameDayCutoff
}

// pickupChangeCutoffInTx resolves the cutoff at the guardian write boundary.
// It deliberately bypasses the request cache so a cutoff changed while this
// request was waiting on its row locks takes effect for the pending write.
func (s *Service) pickupChangeCutoffInTx(ctx context.Context, tenantID int64) (careplan.SameDayCutoff, error) {
	policy, err := s.pickupChangePolicyInTx(ctx, tenantID)
	if err != nil {
		return careplan.SameDayCutoff{}, err
	}
	return policy.cutoff, nil
}

// guardianReasonRequired answers whether the submitting family must state a
// reason (#2267, story 28).
func (s *Service) guardianReasonRequired(ctx context.Context, tenantID int64) bool {
	return s.GuardianReasonRequired(ctx, tenantID)
}

// pickupChangePolicyInTx re-reads the effective parent pickup-change policy
// after taking its shared lock. The corresponding settings writers hold the
// exclusive side until commit, so the enabled flag and cutoff describe one
// committed policy for the duration of the guardian write.
func (s *Service) pickupChangePolicyInTx(ctx context.Context, tenantID int64) (pickupChangePolicy, error) {
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
	cutoff, err := careplan.NewSameDayCutoffAt(clock, s.now)
	if err != nil {
		return pickupChangePolicy{}, fmt.Errorf("parent: %w", err)
	}
	return pickupChangePolicy{enabled: true, cutoff: cutoff}, nil
}
