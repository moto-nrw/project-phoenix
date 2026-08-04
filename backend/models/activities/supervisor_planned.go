package activities

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// SupervisorPlanned represents a staff member assigned to supervise an activity group
type SupervisorPlanned struct {
	base.Model `bun:"schema:activities,table:supervisors"`
	base.TenantModel
	StaffID          int64          `bun:"staff_id,notnull" json:"staff_id"`
	GroupID          int64          `bun:"group_id,notnull" json:"group_id"`
	IsPrimary        bool           `bun:"is_primary,notnull,default:false" json:"is_primary"`
	ValidFrom        timezone.Date  `bun:"valid_from,notnull,default:current_date" json:"valid_from"`
	ValidUntil       *timezone.Date `bun:"valid_until" json:"valid_until,omitempty"`
	CalendarPeriodID *int64         `bun:"calendar_period_id" json:"calendar_period_id,omitempty"`
	// Weekday scopes this assignment to a single ISO weekday (1=Mon … 7=Sun)
	// of the recurring template (issue #2129). NULL means the assignment
	// applies on every weekday the series runs, which is what every row
	// created before per-weekday staffing means. The template writer expands
	// the shared default plus the per-weekday deviations into concrete rows,
	// so readers only need `Weekday == nil || *Weekday == isoWeekday(date)`.
	Weekday *int `bun:"weekday" json:"weekday,omitempty"`

	// Relations - these would be populated when using the ORM's relations
	Staff *users.Staff `bun:"rel:belongs-to,join:staff_id=id" json:"staff,omitempty"`
	Group *Group       `bun:"rel:belongs-to,join:group_id=id" json:"group,omitempty"`
}

// Validate ensures supervisor planned data is valid
func (sp *SupervisorPlanned) Validate() error {
	if sp.StaffID <= 0 {
		return errors.New("staff ID is required")
	}

	if sp.GroupID <= 0 {
		return errors.New("group ID is required")
	}

	if sp.Weekday != nil && !IsValidWeekday(*sp.Weekday) {
		return errors.New("weekday must be between 1 and 7")
	}

	return nil
}

// SetPrimary marks this supervisor as the primary supervisor
func (sp *SupervisorPlanned) SetPrimary() {
	sp.IsPrimary = true
}

// SetNotPrimary marks this supervisor as not the primary supervisor
func (sp *SupervisorPlanned) SetNotPrimary() {
	sp.IsPrimary = false
}
