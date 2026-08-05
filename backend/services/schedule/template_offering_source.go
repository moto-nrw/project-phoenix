package schedule

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// ErrOfferingSourceInvalid identifies a rejected offering-source declaration
// on a timetable template (#2137): unknown/archived offering, an offering
// whose phase does not fit the template's calendar period, or malformed
// filter values. API handlers expose it as a client-correctable 400. The
// enrollment-side resync implementation wraps its validation failures with
// this sentinel so the schedule layer stays free of enrollment imports.
var ErrOfferingSourceInvalid = errors.New("offering source is invalid")

// validateOfferingSourceReference guards the offering-source reference BEFORE
// the group row carrying it is written (#2147 review round 18). Without this
// pre-check an unknown source_care_offering_id fails on the FK during the
// insert/update, which surfaces as a 500 — the offering-source validation
// inside the roster resync never gets to classify it as
// ErrOfferingSourceInvalid (400). The resync stays the authoritative guard;
// this call only pulls the same verdict forward. Read-only test facades may
// leave the hook nil, in which case the resync remains the only check.
func (s *TimetableDataService) validateOfferingSourceReference(
	ctx context.Context,
	offeringID *int64,
	calendarPeriodID *int64,
	op string,
) error {
	if offeringID == nil || s.deps.ValidateOfferingSource == nil {
		return nil
	}
	if err := s.deps.ValidateOfferingSource(ctx, *offeringID, calendarPeriodID); err != nil {
		return &ScheduleError{Op: op, Err: err}
	}
	return nil
}

// OfferingRosterResyncInput describes one template's offering-source rule for
// the roster resync hook (#2137). The hook reconciles the template's
// offering-sourced student_enrollments rows with the offering's currently
// approved enrollments: rows for children that no longer match are capped at
// EffectiveFrom (started) or deleted (not yet effective), missing rows are
// seeded. Rows fed by a legacy CareOffering.ActivityGroupID link onto the
// same template are left untouched.
type OfferingRosterResyncInput struct {
	TemplateID int64
	// PreviousOfferingID is the offering the template was sourced from before
	// this edit (nil on create or when it had no source). Informational: the
	// resync reconciles by diffing every tagged row of the template against
	// the new source's wanted windows, so stale rows are cleaned up without
	// scoping by the previous offering.
	PreviousOfferingID *int64
	// OfferingID is the new source; nil removes the source (cleanup only).
	OfferingID *int64
	// GradeLevels is the Jahrgang filter; empty admits every enrolled child.
	GradeLevels []int
	// CalendarPeriodID is the template's period pin, stamped onto seeded rows
	// and validated against the offering's phase window.
	CalendarPeriodID *int64
	// EffectiveFrom bounds the rewrite: history before it is never touched.
	EffectiveFrom timezone.Date
}
