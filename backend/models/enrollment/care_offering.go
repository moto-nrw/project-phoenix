package enrollment

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// CareOffering is a row in enrollment.care_offerings - one care option
// in the tenant's catalog. Admins build the catalog per calendar period
// (typically a school year, occasionally a holiday); parents pick from
// the open-window subset on the public submission form.
type CareOffering struct {
	base.Model `bun:"schema:enrollment,table:care_offerings"`
	base.TenantModel
	PhaseID             int64    `bun:"phase_id,notnull" json:"phase_id"`
	ActivityGroupID     *int64   `bun:"activity_group_id" json:"activity_group_id,omitempty"`
	Name                string   `bun:"name,notnull" json:"name"`
	Description         *string  `bun:"description" json:"description,omitempty"`
	DaysOfWeekMode      string   `bun:"days_of_week_mode,notnull,default:'fixed'" json:"days_of_week_mode"`
	AvailableDays       []string `bun:"available_days,type:jsonb,notnull" json:"available_days"`
	IncludesHolidayCare bool     `bun:"includes_holiday_care,notnull" json:"includes_holiday_care"`
	IncludesLunch       bool     `bun:"includes_lunch,notnull" json:"includes_lunch"`
	Capacity            *int     `bun:"capacity" json:"capacity,omitempty"`
	PriceCents          *int     `bun:"price_cents" json:"price_cents,omitempty"`
	IsActive            bool     `bun:"is_active,notnull" json:"is_active"`
	IsRequired          bool     `bun:"is_required,notnull,default:false" json:"is_required"`
	SortOrder           int      `bun:"sort_order,notnull,default:0" json:"sort_order"`
}

// TableName returns the schema-qualified table name.
func (c *CareOffering) TableName() string {
	return "enrollment.care_offerings"
}

// Validate enforces the column-level CHECK constraints in app code so
// we fail fast before the round-trip. Service / application windows
// moved to the owning Phase; this struct only owns offering-level
// fields now (capacity, days, lunch, etc.).
func (c *CareOffering) Validate() error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return errors.New("care offering name is required")
	}
	if c.PhaseID == 0 {
		return errors.New("phase_id is required")
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
	// A required offering must be available to every child, so it cannot
	// carry a hard capacity limit - otherwise a full offering would block
	// every new enrollment in the phase. The admin editor prevents the
	// combination; this is the backstop.
	if c.IsRequired && c.Capacity != nil {
		return errors.New("a required care offering must not have a capacity limit")
	}
	return nil
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

	// ListByPhase returns offerings filtered to a single phase -
	// admin pages pivot on the phase dropdown, and the public-form
	// endpoint uses it after resolving the parent's selected phase.
	ListByPhase(ctx context.Context, phaseID int64) ([]*CareOffering, error)

	// ListActiveByPhase returns is_active=true offerings for a phase.
	// Used by the parent-facing endpoint; the phase's own enrollment
	// window gates the surrounding flow, so individual offerings
	// don't need their own time filter.
	ListActiveByPhase(ctx context.Context, phaseID int64) ([]*CareOffering, error)

	// CountByPhaseID returns how many care offerings belong to the phase.
	// Powers the phase-delete confirmation modal.
	CountByPhaseID(ctx context.Context, phaseID int64) (int, error)
}

// RequestChildOffering is a row in enrollment.request_child_offerings -
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

	// CountActiveByCareOffering returns the number of children currently
	// holding (or competing for) a slot in the given care offering.
	// Counts non-terminal statuses on the joined request_children row:
	// submitted, under_review, approved, waitlisted. Excludes rejected
	// and withdrawn. Used for capacity-overflow enforcement at submit
	// time.
	CountActiveByCareOffering(ctx context.Context, careOfferingID int64) (int, error)
}
