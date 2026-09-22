package schoolcalendar

import (
	"context"
	"errors"
)

var (
	// ErrCalendarPeriodOverlapConflict is returned when adding or changing an
	// active period would overlap an existing active period of the same
	// period_type (#1837). Overlaps across different types (holidays inside a
	// school year) stay legal and are only surfaced as advisory warnings.
	ErrCalendarPeriodOverlapConflict = errors.New("calendar period overlaps an active period of the same type")
	// ErrCalendarPeriodRequiredByCareOffering is returned when changing or
	// removing a period would invalidate a care offering's linked timetable
	// series. The period stays unchanged until the offering is relinked or
	// removed.
	ErrCalendarPeriodRequiredByCareOffering = errors.New("calendar period is required by a linked care offering")
)

// CalendarPeriodOverlapError wraps ErrCalendarPeriodOverlapConflict and
// carries the conflicting periods so handlers can name them in the 409
// response. errors.Is against the sentinel keeps matching through Unwrap.
type CalendarPeriodOverlapError struct {
	Overlaps []CalendarPeriod
}

func (e *CalendarPeriodOverlapError) Error() string { return ErrCalendarPeriodOverlapConflict.Error() }
func (e *CalendarPeriodOverlapError) Unwrap() error { return ErrCalendarPeriodOverlapConflict }

// CalendarPeriodAdministration is the administrative write path of the
// school calendar, the one the period editor drives. Unlike the plain
// Command writes it enforces the school's rules: an active period may not
// overlap another active period of the same type, names stay unique, every
// mutation runs under the tenant recurrence gate that serializes it against
// materialization and template re-planning, and a change or removal is
// refused while a linked care offering still needs the period.
type CalendarPeriodAdministration interface {
	// AddCalendarPeriod creates a period after the name and same-type overlap
	// checks; both run under the recurrence gate so two concurrent adds
	// cannot both pass the overlap check before either insert commits.
	AddCalendarPeriod(context.Context, CreateCalendarPeriod) (CalendarPeriod, error)
	// ChangeCalendarPeriod replaces a period's fields. The overlap rule only
	// guards changes to the scheduling-relevant fields (dates, active flag
	// and type); a rename of a period that already overlaps stays possible.
	ChangeCalendarPeriod(context.Context, UpdateCalendarPeriod) (CalendarPeriod, error)
	// RemoveCalendarPeriod deletes a period the care offerings no longer need.
	RemoveCalendarPeriod(context.Context, int64) error
	// EnsureDefaultSchoolYear guarantees the tenant has at least one calendar
	// period. If none exists, it creates the current German school year
	// (Aug 1 – Jul 31) as an active period. Idempotent and race-safe; created
	// reports whether this call inserted the default period.
	EnsureDefaultSchoolYear(context.Context) (periods []CalendarPeriod, created bool, err error)
	// ListActiveOverlaps returns the active periods whose date range overlaps
	// the given period (excluding the period itself). Returns nil when the
	// period is inactive — only active/active collisions are advisory-worthy.
	ListActiveOverlaps(context.Context, CalendarPeriod) ([]CalendarPeriod, error)
}

// Calendar is the whole School Calendar owner surface: the persistence-level
// Capability, the administrative period write path and the tenant
// non-working-day reads. Composition roots hand it to the consumers that
// administer the calendar; narrower consumers name only the slice they use.
type Calendar interface {
	Capability
	CalendarPeriodAdministration
	NonWorkingDayQuery
}

func (m *Module) AddCalendarPeriod(ctx context.Context, input CreateCalendarPeriod) (CalendarPeriod, error) {
	if err := validateCalendarPeriodFields(&input.CalendarPeriodFields); err != nil {
		return CalendarPeriod{}, err
	}
	return m.engine.AddCalendarPeriod(ctx, input)
}

func (m *Module) ChangeCalendarPeriod(ctx context.Context, input UpdateCalendarPeriod) (CalendarPeriod, error) {
	if input.ID <= 0 {
		return CalendarPeriod{}, invalidPeriod("calendar period ID is required")
	}
	if err := validateCalendarPeriodFields(&input.CalendarPeriodFields); err != nil {
		return CalendarPeriod{}, err
	}
	return m.engine.ChangeCalendarPeriod(ctx, input)
}

func (m *Module) RemoveCalendarPeriod(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidPeriod("calendar period ID is required")
	}
	return m.engine.RemoveCalendarPeriod(ctx, id)
}

func (m *Module) EnsureDefaultSchoolYear(ctx context.Context) ([]CalendarPeriod, bool, error) {
	return m.engine.EnsureDefaultSchoolYear(ctx)
}

// ListActiveOverlaps is advisory only: callers attach the result as a
// warning, never as a save blocker. Inactive periods cannot collide with
// anything, so the store round-trip is skipped for them.
func (m *Module) ListActiveOverlaps(ctx context.Context, period CalendarPeriod) ([]CalendarPeriod, error) {
	if !period.IsActive {
		return nil, nil
	}
	return m.ListCalendarPeriods(ctx, CalendarPeriodFilter{
		ActiveOnly: true, OverlappingFrom: period.StartDate, OverlappingTo: period.EndDate, ExcludeID: period.ID,
	})
}
