package enrollment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Days-of-week mode values matching the column CHECK constraint.
const (
	DaysOfWeekModeFixed        = "fixed"
	DaysOfWeekModeParentChoice = "parent_choice"
)

// validDaysOfWeekModes mirrors the CHECK constraint from the migration.
var validDaysOfWeekModes = map[string]bool{
	DaysOfWeekModeFixed:        true,
	DaysOfWeekModeParentChoice: true,
}

// canonicalDaySet enumerates the allowed values inside available_days
// + selected_days. Lowercase three-letter abbreviations match the
// project's German-context norm (e.g., the activity scheduler).
var canonicalDaySet = map[string]bool{
	"mon": true, "tue": true, "wed": true, "thu": true, "fri": true,
	"sat": true, "sun": true,
}

// CareOffering is a row in enrollment.care_offerings — one care option
// in the tenant's catalog. Admins build the catalog per calendar period
// (typically a school year, occasionally a holiday); parents pick from
// the open-window subset on the public submission form.
type CareOffering struct {
	base.Model `bun:"schema:enrollment,table:care_offerings"`
	base.TenantModel
	CalendarPeriodID       *int64     `bun:"calendar_period_id" json:"calendar_period_id,omitempty"`
	ActivityGroupID        *int64     `bun:"activity_group_id" json:"activity_group_id,omitempty"`
	Name                   string     `bun:"name,notnull" json:"name"`
	Description            *string    `bun:"description" json:"description,omitempty"`
	DaysOfWeekMode         string     `bun:"days_of_week_mode,notnull,default:'fixed'" json:"days_of_week_mode"`
	AvailableDays          []string   `bun:"available_days,type:jsonb,notnull,default:'[\"mon\",\"tue\",\"wed\",\"thu\",\"fri\"]'" json:"available_days"`
	IncludesHolidayCare    bool       `bun:"includes_holiday_care,notnull,default:false" json:"includes_holiday_care"`
	IncludesLunch          bool       `bun:"includes_lunch,notnull,default:false" json:"includes_lunch"`
	Capacity               *int       `bun:"capacity" json:"capacity,omitempty"`
	PriceCents             *int       `bun:"price_cents" json:"price_cents,omitempty"`
	ApplicationWindowStart *time.Time `bun:"application_window_start" json:"application_window_start,omitempty"`
	ApplicationWindowEnd   *time.Time `bun:"application_window_end" json:"application_window_end,omitempty"`
	ServiceStartDate       *time.Time `bun:"service_start_date,type:date" json:"service_start_date,omitempty"`
	ServiceEndDate         *time.Time `bun:"service_end_date,type:date" json:"service_end_date,omitempty"`
	IsActive               bool       `bun:"is_active,notnull,default:true" json:"is_active"`
	SortOrder              int        `bun:"sort_order,notnull,default:0" json:"sort_order"`
}

// TableName returns the schema-qualified table name.
func (c *CareOffering) TableName() string {
	return "enrollment.care_offerings"
}

// Validate enforces the column-level CHECK constraints in app code so
// we fail fast before the round-trip. Mirrors the CHECK clauses on the
// table from migration 1.15.52.
func (c *CareOffering) Validate() error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return errors.New("care offering name is required")
	}
	if c.DaysOfWeekMode == "" {
		c.DaysOfWeekMode = DaysOfWeekModeFixed
	}
	if !validDaysOfWeekModes[c.DaysOfWeekMode] {
		return fmt.Errorf("days_of_week_mode must be 'fixed' or 'parent_choice', got %q", c.DaysOfWeekMode)
	}
	for _, d := range c.AvailableDays {
		if !canonicalDaySet[strings.ToLower(d)] {
			return fmt.Errorf("available_days entry %q is not a known day abbreviation", d)
		}
	}
	if c.Capacity != nil && *c.Capacity < 0 {
		return errors.New("capacity must be non-negative")
	}
	if c.PriceCents != nil && *c.PriceCents < 0 {
		return errors.New("price_cents must be non-negative")
	}
	if c.ApplicationWindowStart != nil && c.ApplicationWindowEnd != nil &&
		!c.ApplicationWindowEnd.After(*c.ApplicationWindowStart) {
		return errors.New("application_window_end must be after application_window_start")
	}
	if c.ServiceStartDate != nil && c.ServiceEndDate != nil &&
		c.ServiceEndDate.Before(*c.ServiceStartDate) {
		return errors.New("service_end_date must be on or after service_start_date")
	}
	return nil
}

// IsApplicationWindowOpen returns true when `now` falls within the
// application window (NULL bounds = unbounded). Used by the parent-
// facing endpoint and as a defense-in-depth check on submission.
func (c *CareOffering) IsApplicationWindowOpen(now time.Time) bool {
	if c.ApplicationWindowStart != nil && now.Before(*c.ApplicationWindowStart) {
		return false
	}
	if c.ApplicationWindowEnd != nil && !now.Before(*c.ApplicationWindowEnd) {
		// `!now.Before(end)` == `now >= end` — the window is half-open;
		// the moment `end` arrives, the offering is closed.
		return false
	}
	return true
}

// HasUnlimitedCapacity returns true when the capacity field is NULL,
// matching the "null = unlimited" semantics on the column.
func (c *CareOffering) HasUnlimitedCapacity() bool {
	return c.Capacity == nil
}

// CareOfferingRepository is the DB contract used by the service layer.
type CareOfferingRepository interface {
	Create(ctx context.Context, offering *CareOffering) error
	FindByID(ctx context.Context, id int64) (*CareOffering, error)
	Update(ctx context.Context, offering *CareOffering) error
	Delete(ctx context.Context, id int64) error

	// ListByTenant returns every care offering for the tenant in
	// context, sorted by sort_order. Admin endpoint uses this.
	ListByTenant(ctx context.Context) ([]*CareOffering, error)

	// ListByCalendarPeriod returns offerings filtered to a single
	// calendar period — admin pages typically pivot on the period
	// dropdown.
	ListByCalendarPeriod(ctx context.Context, calendarPeriodID int64) ([]*CareOffering, error)

	// ListPublicOpenWindow returns offerings the public form should
	// expose: is_active=true AND the application window includes `now`.
	// Used by the parent-facing endpoint; defense-in-depth for the
	// submission service in PR 7.
	ListPublicOpenWindow(ctx context.Context, now time.Time) ([]*CareOffering, error)
}

// RequestChildOffering is a row in enrollment.request_child_offerings —
// the join table linking a request_child to a care_offering. PR 6
// ships the schema; PR 7 fills it on submission.
type RequestChildOffering struct {
	base.Model `bun:"schema:enrollment,table:request_child_offerings"`
	base.TenantModel
	RequestChildID int64    `bun:"request_child_id,notnull" json:"request_child_id"`
	CareOfferingID int64    `bun:"care_offering_id,notnull" json:"care_offering_id"`
	SelectedDays   []string `bun:"selected_days,type:jsonb,nullzero" json:"selected_days,omitempty"`
	Notes          *string  `bun:"notes" json:"notes,omitempty"`
}

func (r *RequestChildOffering) TableName() string {
	return "enrollment.request_child_offerings"
}

// RequestChildOfferingRepository is the contract PR 7's submission
// service consumes. PR 6 only ships the type so the factory can wire
// it; the implementation has no callers yet.
type RequestChildOfferingRepository interface {
	Create(ctx context.Context, row *RequestChildOffering) error
	ListByRequestChildID(ctx context.Context, requestChildID int64) ([]*RequestChildOffering, error)
}
