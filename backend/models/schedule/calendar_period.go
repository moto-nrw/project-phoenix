package schedule

import (
	"context"
	"errors"
	"time"
)

// Period type constants
const (
	PeriodTypeSchoolYear = "school_year"
	PeriodTypeSemester   = "semester"
	PeriodTypeHoliday    = "holiday"
	PeriodTypeCustom     = "custom"
)

// Sentinel errors for the calendar period domain. Callers should compare via
// errors.Is rather than matching on error strings.
var (
	// ErrCalendarPeriodNameConflict is returned when a create or update would
	// violate the per-tenant name uniqueness rule.
	ErrCalendarPeriodNameConflict = errors.New("calendar period name already exists")
	// ErrCalendarPeriodCareOfferingConflict is returned when changing or
	// deleting a period would invalidate a care offering's linked timetable
	// series. The caller must keep the period unchanged until the offering is
	// relinked or removed.
	ErrCalendarPeriodCareOfferingConflict = errors.New("calendar period is required by a linked care offering")
	// ErrCalendarPeriodOverlapConflict is returned when creating or updating
	// an active period would overlap an existing active period of the same
	// period_type (#1837). Overlaps across different types (e.g. holidays
	// inside a school year) stay legal and are only surfaced as advisory
	// warnings.
	ErrCalendarPeriodOverlapConflict = errors.New("calendar period overlaps an active period of the same type")
)

// CalendarPeriodOverlapError wraps ErrCalendarPeriodOverlapConflict and
// carries the conflicting periods so handlers can name them in the 409
// response. errors.Is against the sentinel keeps matching through Unwrap.
type CalendarPeriodOverlapError struct {
	Overlaps []*CalendarPeriod
}

func (e *CalendarPeriodOverlapError) Error() string {
	return ErrCalendarPeriodOverlapConflict.Error()
}

func (e *CalendarPeriodOverlapError) Unwrap() error {
	return ErrCalendarPeriodOverlapConflict
}

// CalendarPeriodNameMaxLength is the maximum length of the name field.
const CalendarPeriodNameMaxLength = 255

// CalendarPeriod represents a time period in the school calendar (e.g. school year, semester, holiday).
type CalendarPeriod struct {
	Model `bun:"schema:schedule,table:calendar_periods"`
	TenantModel

	Name            string `bun:"name,notnull" json:"name"`
	PeriodType      string `bun:"period_type,notnull" json:"period_type"`
	StartDate       Date   `bun:"start_date,notnull" json:"start_date"`
	EndDate         Date   `bun:"end_date,notnull" json:"end_date"`
	WeekCycleLength int    `bun:"week_cycle_length,notnull,default:1" json:"week_cycle_length"`
	WeekCycleAnchor *Date  `bun:"week_cycle_anchor" json:"week_cycle_anchor,omitempty"`
	IsActive        bool   `bun:"is_active,notnull,default:false" json:"is_active"`
}

// Validate ensures calendar period data is valid
func (p *CalendarPeriod) Validate() error {
	if p.Name == "" {
		return errors.New("name is required")
	}
	if len(p.Name) > CalendarPeriodNameMaxLength {
		return errors.New("name cannot exceed 255 characters")
	}
	if !IsValidPeriodType(p.PeriodType) {
		return errors.New("invalid period type")
	}
	if p.StartDate.IsZero() {
		return errors.New("start_date is required")
	}
	if p.EndDate.IsZero() {
		return errors.New("end_date is required")
	}
	if !p.EndDate.After(p.StartDate) {
		return errors.New("end_date must be after start_date")
	}
	if p.WeekCycleLength < 1 {
		return errors.New("week_cycle_length must be at least 1")
	}
	if p.WeekCycleLength > 1 && p.WeekCycleAnchor == nil {
		return errors.New("week_cycle_anchor is required when week_cycle_length > 1")
	}
	return nil
}

// IsValidPeriodType checks if the given string is a valid period type
func IsValidPeriodType(t string) bool {
	switch t {
	case PeriodTypeSchoolYear, PeriodTypeSemester, PeriodTypeHoliday, PeriodTypeCustom:
		return true
	}
	return false
}

// HasWeekCycle returns true if this period uses A/B week alternation
func (p *CalendarPeriod) HasWeekCycle() bool {
	return p.WeekCycleLength > 1
}

// ContainsDay returns true if the given calendar day falls within this
// period (inclusive on both ends).
func (p *CalendarPeriod) ContainsDay(d Date) bool {
	return !d.Before(p.StartDate) && !d.After(p.EndDate)
}

// ContainsDate returns true if the Berlin calendar day of the given instant
// falls within this period. Callers that already hold a Date should
// use ContainsDay directly.
func (p *CalendarPeriod) ContainsDate(date time.Time) bool {
	return p.ContainsDay(DateFromTime(date))
}

// CalendarPeriodUsage aggregates how many planning objects reference a
// calendar period. Most FKs are nullable and get cleared on delete, but roster
// rows can still make deletion fail when clearing calendar_period_id would
// collide with an existing unscoped active assignment.
type CalendarPeriodUsage struct {
	EnrollmentPhases   int
	ActivityGroups     int
	Schedules          int
	StudentEnrollments int
	Supervisors        int
	ActivityInstances  int
}

// CalendarPeriodRepository defines operations for managing calendar periods
type CalendarPeriodRepository interface {
	repository[*CalendarPeriod]

	// FindByTenantID finds all calendar periods for a tenant
	FindByTenantID(ctx context.Context) ([]*CalendarPeriod, error)

	// FindActiveByTenantID finds all active calendar periods for a tenant
	FindActiveByTenantID(ctx context.Context) ([]*CalendarPeriod, error)

	// FindByName finds a calendar period by name within the current tenant
	FindByName(ctx context.Context, name string) (*CalendarPeriod, error)

	// CreateIfAbsent inserts the period unless a row with the same
	// (tenant_id, name) already exists. It reports whether the insert
	// happened (false = a period with that name was already present).
	CreateIfAbsent(ctx context.Context, p *CalendarPeriod) (bool, error)

	// FindActiveOverlapping returns all active periods of the current tenant
	// whose [start_date, end_date] range overlaps [start, end] (inclusive on
	// both ends), excluding the period with excludeID.
	FindActiveOverlapping(ctx context.Context, start, end Date, excludeID int64) ([]*CalendarPeriod, error)

	// FindActiveOverlappingByType behaves like FindActiveOverlapping but only
	// considers active periods of the given period_type. It backs the hard
	// same-type overlap rejection; the untyped variant stays advisory.
	FindActiveOverlappingByType(ctx context.Context, periodType string, start, end Date, excludeID int64) ([]*CalendarPeriod, error)

	// UsageCounts returns, per calendar period of the current tenant, how many
	// rows reference it through nullable calendar_period_id FKs. Periods without
	// references are omitted from the map.
	UsageCounts(ctx context.Context) (map[int64]CalendarPeriodUsage, error)
}
