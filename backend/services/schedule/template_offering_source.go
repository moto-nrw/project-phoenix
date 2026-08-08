package schedule

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// ErrOfferingSourceInvalid identifies a rejected offering-source declaration
// on a timetable template (#2137): unknown/archived offering, an offering
// whose phase does not fit the template's calendar period, source offerings
// from different enrollment phases, or malformed filter values. API handlers
// expose it as a client-correctable 400. The enrollment-side resync
// implementation wraps its validation failures with this sentinel so the
// schedule layer stays free of enrollment imports.
var ErrOfferingSourceInvalid = errors.New("offering source is invalid")

// MaxOfferingSourcesPerTemplate caps how many source offerings one template
// may union. The validation and resync paths resolve every id individually,
// and the id list arrives both from the editor (combined-counts preview,
// template save) and from the stored jsonb array on every later resync — an
// unbounded list would turn one request into thousands of sequential queries
// inside a tenant transaction. Real schools run a handful of Betreuungs-
// angebote; 50 is far above any legitimate selection.
const MaxOfferingSourcesPerTemplate = 50

// validateOfferingSourceReference guards the offering-source references
// BEFORE the group row carrying them is written (#2147 review round 18).
// Without this pre-check an invalid source id set fails only inside the
// roster resync AFTER the row write; pulling the verdict forward keeps the
// 400/500 classification clean and rejects same-phase violations before any
// write happens. The resync stays the authoritative guard; this call only
// pulls the same verdict forward. Read-only test facades may leave the hook
// nil, in which case the resync remains the only check.
func (s *TimetableDataService) validateOfferingSourceReference(
	ctx context.Context,
	offeringIDs []int64,
	calendarPeriodID *int64,
	op string,
) error {
	if len(offeringIDs) == 0 || s.deps.ValidateOfferingSource == nil {
		return nil
	}
	if err := s.deps.ValidateOfferingSource(ctx, offeringIDs, calendarPeriodID); err != nil {
		return &ScheduleError{Op: op, Err: err}
	}
	return nil
}

// OfferingRosterResyncInput describes one template's offering-source rule for
// the roster resync hook (#2137). The hook reconciles the template's
// offering-sourced student_enrollments rows with the union of the source
// offerings' currently approved enrollments: rows for children that no longer
// match are capped at EffectiveFrom (started) or deleted (not yet effective),
// missing rows are seeded. Rows fed by a legacy CareOffering.ActivityGroupID
// link onto the same template are left untouched.
type OfferingRosterResyncInput struct {
	TemplateID int64
	// OfferingIDs are the source offerings in stored array order; the resync
	// unions their approved children, subtracting later offerings' overlapping
	// weekday×date contributions so no child is ever planned twice. All
	// offerings must belong to the same enrollment phase. Empty removes the
	// source (cleanup only). The resync reconciles by diffing every tagged row
	// of the template against the union's wanted windows, so stale rows of a
	// previous source set are cleaned up without naming it.
	OfferingIDs []int64
	// GradeLevels is the Jahrgang filter; empty admits every enrolled child.
	GradeLevels []int
	// CalendarPeriodID is the template's period pin, stamped onto seeded rows
	// and validated against the offerings' phase window.
	CalendarPeriodID *int64
	// EffectiveFrom bounds the rewrite: history before it is never touched.
	EffectiveFrom timezone.Date
}
