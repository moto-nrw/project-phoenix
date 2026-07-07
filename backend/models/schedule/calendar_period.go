package schedule

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
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
)

// tableCalendarPeriods is the schema-qualified table name.
const tableCalendarPeriods = "schedule.calendar_periods"

// CalendarPeriodNameMaxLength is the maximum length of the name field.
const CalendarPeriodNameMaxLength = 255

// CalendarPeriod represents a time period in the school calendar (e.g. school year, semester, holiday).
type CalendarPeriod struct {
	base.Model `bun:"schema:schedule,table:calendar_periods"`
	base.TenantModel

	Name            string         `bun:"name,notnull" json:"name"`
	PeriodType      string         `bun:"period_type,notnull" json:"period_type"`
	StartDate       timezone.Date  `bun:"start_date,notnull" json:"start_date"`
	EndDate         timezone.Date  `bun:"end_date,notnull" json:"end_date"`
	WeekCycleLength int            `bun:"week_cycle_length,notnull,default:1" json:"week_cycle_length"`
	WeekCycleAnchor *timezone.Date `bun:"week_cycle_anchor" json:"week_cycle_anchor,omitempty"`
	IsActive        bool           `bun:"is_active,notnull,default:false" json:"is_active"`
}

func (p *CalendarPeriod) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`schedule.calendar_periods AS "calendar_period"`)
	}
	return nil
}

// TableName returns the database table name
func (p *CalendarPeriod) TableName() string {
	return tableCalendarPeriods
}

// GetID implements the Entity interface
func (p *CalendarPeriod) GetID() any {
	return p.ID
}

// GetCreatedAt implements the Entity interface
func (p *CalendarPeriod) GetCreatedAt() time.Time {
	return p.CreatedAt
}

// GetUpdatedAt implements the Entity interface
func (p *CalendarPeriod) GetUpdatedAt() time.Time {
	return p.UpdatedAt
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
func (p *CalendarPeriod) ContainsDay(d timezone.Date) bool {
	return !d.Before(p.StartDate) && !d.After(p.EndDate)
}

// ContainsDate returns true if the Berlin calendar day of the given instant
// falls within this period. Callers that already hold a timezone.Date should
// use ContainsDay directly.
func (p *CalendarPeriod) ContainsDate(date time.Time) bool {
	return p.ContainsDay(timezone.DateFromTime(date))
}

// CalendarPeriodUsage aggregates how many planning objects reference a
// calendar period. Most FKs are nullable and get cleared on delete, but roster
// rows can still make deletion fail when clearing calendar_period_id would
// collide with an existing unscoped active assignment.
type CalendarPeriodUsage struct {
	EnrollmentPhases   int
	Schedules          int
	StudentEnrollments int
	Supervisors        int
	ActivityInstances  int
}

// CalendarPeriodRepository defines operations for managing calendar periods
type CalendarPeriodRepository interface {
	base.Repository[*CalendarPeriod]

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
	FindActiveOverlapping(ctx context.Context, start, end timezone.Date, excludeID int64) ([]*CalendarPeriod, error)

	// UsageCounts returns, per calendar period of the current tenant, how many
	// rows reference it through nullable calendar_period_id FKs. Periods without
	// references are omitted from the map.
	UsageCounts(ctx context.Context) (map[int64]CalendarPeriodUsage, error)
}
