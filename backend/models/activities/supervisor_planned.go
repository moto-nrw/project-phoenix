package activities

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// SupervisorQueryOptions and SupervisorDate keep the temporary compatibility
// repository contract self-contained while callers migrate to Timetable.
type SupervisorDate = timezone.Date

// SupervisorPlanned represents a staff member assigned to supervise an activity group
type SupervisorPlanned struct {
	Model `bun:"schema:activities,table:supervisors"`
	TenantModel
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

	StaffPersonID int64  `bun:"-" json:"-"`
	FirstName     string `bun:"-" json:"first_name,omitempty"`
	LastName      string `bun:"-" json:"last_name,omitempty"`
	Group         *Group `bun:"rel:belongs-to,join:group_id=id" json:"group,omitempty"`
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
	sp.ValidFrom = timezone.Date(validFrom)
	sp.ValidUntil = scheduleDate(validUntil)
}
