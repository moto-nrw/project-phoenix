package parent

import (
	"context"
	"fmt"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// pickupChangeCutoff resolves the same-day cutoff for a guardian's one-day
// pickup change (#3163) for the child's school, judged at the service clock.
// The cutoff only takes effect while the one-day pickup change is switched on
// for that school; otherwise the setting is hidden and the zero cutoff applies.
func (s *service) pickupChangeCutoff(ctx context.Context, tenantID int64) (scheduleService.SameDayCutoff, error) {
	now := s.now()
	enabled, err := s.Settings.ResolveBoolForTenant(ctx, tenantID, configModels.KeyParentPickupChangeEnabled)
	if err != nil {
		return scheduleService.SameDayCutoff{}, fmt.Errorf("parent: resolve pickup-change setting: %w", err)
	}
	if !enabled {
		return scheduleService.SameDayCutoff{Now: now}, nil
	}
	clock, err := s.Settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyParentPickupChangeCutoffTime)
	if err != nil {
		return scheduleService.SameDayCutoff{}, fmt.Errorf("parent: resolve pickup-change cutoff: %w", err)
	}
	cutoff, err := scheduleService.NewSameDayCutoff(clock, now)
	if err != nil {
		return scheduleService.SameDayCutoff{}, fmt.Errorf("parent: %w", err)
	}
	return cutoff, nil
}
