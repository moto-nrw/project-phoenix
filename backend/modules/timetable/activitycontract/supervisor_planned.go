package activitycontract

import (
	"errors"
	"time"
)

// SupervisorQueryOptions and Date keep the temporary compatibility
// repository contract self-contained while callers migrate to Timetable.

// SupervisorPlanned represents a staff member assigned to supervise an activity group
type SupervisorPlanned struct {
	ID               int64     `json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	TenantID         int64     `json:"tenant_id"`
	StaffID          int64     `json:"staff_id"`
	GroupID          int64     `json:"group_id"`
	IsPrimary        bool      `json:"is_primary"`
	ValidFrom        Date      `json:"valid_from"`
	ValidUntil       *Date     `json:"valid_until,omitempty"`
	CalendarPeriodID *int64    `json:"calendar_period_id,omitempty"`
	// Weekday scopes this assignment to a single ISO weekday (1=Mon … 7=Sun)
	// of the recurring template (issue #2129). NULL means the assignment
	// applies on every weekday the series runs, which is what every row
	// created before per-weekday staffing means. The template writer expands
	// the shared default plus the per-weekday deviations into concrete rows,
	// so readers only need `Weekday == nil || *Weekday == isoWeekday(date)`.
	Weekday *int `json:"weekday,omitempty"`

	StaffPersonID int64  `json:"-"`
	FirstName     string `json:"first_name,omitempty"`
	LastName      string `json:"last_name,omitempty"`
	Group         *Group `json:"group,omitempty"`
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

func (sp *SupervisorPlanned) ValidityDateStrings() (string, *string) {
	return sp.ValidFrom.String(), scheduleDateString(sp.ValidUntil)
}

func (sp *SupervisorPlanned) SetValidityDateStrings(validFrom string, validUntil *string) {
	sp.ValidFrom = Date(validFrom)
	sp.ValidUntil = scheduleDate(validUntil)
}

func (v *SupervisorPlanned) GetTenantID() int64   { return v.TenantID }
func (v *SupervisorPlanned) SetTenantID(id int64) { v.TenantID = id }
