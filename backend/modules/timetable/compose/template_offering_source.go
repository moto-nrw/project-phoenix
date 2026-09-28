package compose

import (
	"context"
)

// validateOfferingSourceReference guards the offering-source references
// BEFORE the group row carrying them is written (#2147 review round 18).
// Without this pre-check an invalid source id set fails only inside the
// roster resync AFTER the row write; pulling the verdict forward keeps the
// 400/500 classification clean and rejects same-phase violations before any
// write happens. storedOfferingIDs are the template's currently persisted
// ids (nil on create) — a vanished id is tolerated only when stored, a newly
// submitted unknown id rejects (see CareOfferingSeriesValidator). The resync
// stays the authoritative guard; this call only pulls the same verdict
// forward. Read-only test facades may leave the hook nil, in which case the
// resync remains the only check.
func (s *TemplateService) validateOfferingSourceReference(
	ctx context.Context,
	offeringIDs []int64,
	storedOfferingIDs []int64,
	calendarPeriodID *int64,
	op string,
) error {
	if len(offeringIDs) == 0 || s.deps.CareOfferings.ValidateOfferingSource == nil {
		return nil
	}
	if err := s.deps.CareOfferings.ValidateOfferingSource(ctx, offeringIDs, storedOfferingIDs, calendarPeriodID); err != nil {
		return &ScheduleError{Op: op, Err: err}
	}
	return nil
}
