package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Care Plan's booking materialization (#3560) reads and writes the Timetable,
// Enrollment, People Directory and Audit Platform through its own ports. The
// adapters below bind them over the owners' public capabilities with the
// queries the retained repositories issued; they decide nothing.

// timetableOfferingRosterResync serves the Timetable owner's roster resync
// hook (#2137) from Care Plan's booking materialization.
func timetableOfferingRosterResync(rosters careplan.SourcedRosters) func(context.Context, timetable.OfferingRosterResyncInput) error {
	return func(ctx context.Context, in timetable.OfferingRosterResyncInput) error {
		return rosters.ResyncTemplateOfferingRoster(ctx, careplan.OfferingRosterResync{
			TemplateID: in.TemplateID, OfferingIDs: in.OfferingIDs, GradeLevels: in.GradeLevels,
			SchoolClasses: in.SchoolClasses, CalendarPeriodID: in.CalendarPeriodID, EffectiveFrom: in.EffectiveFrom,
			ScopeRequestChildIDs: in.ScopeRequestChildIDs, TolerateDriftedSources: in.TolerateDriftedSources,
			StoredOfferingIDs: in.StoredOfferingIDs,
		})
	}
}
