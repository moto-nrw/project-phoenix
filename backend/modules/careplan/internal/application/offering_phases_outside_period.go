package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// OfferingPhasesOutsidePeriod names the phases whose active offerings the
// planning period leaves out (#5344): their service window overlaps the
// period without lying inside it, so ValidatePhaseWithinPeriod keeps their
// offerings out of the source choice and the booking checks. The planner
// shows them instead of an empty list that reads as "no problems". Four
// reads: the setting, the offerings, the phases and the period.
func (m *BookingMaterialization) OfferingPhasesOutsidePeriod(ctx context.Context, calendarPeriodID int64) ([]careplan.OfferingPhaseOutsidePeriod, error) {
	enabled, err := m.careOfferingsEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("offering phases outside period: resolve care offerings setting: %w", err)
	}
	if !enabled {
		return []careplan.OfferingPhaseOutsidePeriod{}, nil
	}
	offerings, err := m.catalog.listTenantRecords(ctx)
	if err != nil {
		return nil, fmt.Errorf("offering phases outside period: list offerings: %w", err)
	}
	phases, err := m.deps.Enrollment.Phases(ctx)
	if err != nil {
		return nil, fmt.Errorf("offering phases outside period: list phases: %w", err)
	}
	period, err := m.offeringSourcePeriod(ctx, &calendarPeriodID, "offering phases outside period")
	if err != nil {
		return nil, err
	}
	return offeringPhasesOutsidePeriod(phases, offerings, period), nil
}

// offeringPhasesOutsidePeriod keeps the phases with at least one active
// offering whose service window overlaps the period but does not fit it.
// A phase fully before or after the period belongs to another planning
// period and is no exclusion; a nil period excludes nothing, as
// ValidatePhaseWithinPeriod lets every phase pass then. Sorted by service
// start, then name.
func offeringPhasesOutsidePeriod(
	phases []careplan.OfferingPhase,
	offerings []careplan.CareOffering,
	period *careplan.LinkedPeriod,
) []careplan.OfferingPhaseOutsidePeriod {
	result := []careplan.OfferingPhaseOutsidePeriod{}
	if period == nil {
		return result
	}
	active := make(map[int64]int, len(phases))
	for _, offering := range offerings {
		if offering.IsActive {
			active[offering.PhaseID]++
		}
	}
	for _, phase := range phases {
		overlaps := !phase.ServiceStart.After(period.EndDate) && !phase.ServiceEnd.Before(period.StartDate)
		if active[phase.ID] == 0 || !overlaps || careplan.ValidatePhaseWithinPeriod(phase, period) == nil {
			continue
		}
		result = append(result, careplan.OfferingPhaseOutsidePeriod{Phase: phase, OfferingCount: active[phase.ID], Period: *period})
	}
	slices.SortFunc(result, func(a, b careplan.OfferingPhaseOutsidePeriod) int {
		if byStart := a.Phase.ServiceStart.Compare(b.Phase.ServiceStart); byStart != 0 {
			return byStart
		}
		return strings.Compare(a.Phase.Name, b.Phase.Name)
	})
	return result
}
