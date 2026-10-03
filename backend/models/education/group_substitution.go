package education

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

const (
	GroupSubstitutionTypeGroupHandover = "group_handover"
	GroupSubstitutionTypeLegacy        = "legacy_personnel_substitution"
)

// GroupSubstitution represents a temporary substitution of a staff member for another in a group
type GroupSubstitution struct {
	Model
	TenantModel
	TargetType        string        `bun:"target_type,notnull" json:"-"`
	GroupID           int64         `bun:"group_id,notnull" json:"group_id"`
	RegularStaffID    *int64        `bun:"regular_staff_id" json:"regular_staff_id,omitempty"`
	SubstituteStaffID int64         `bun:"substitute_staff_id,notnull" json:"substitute_staff_id"`
	StartDate         calendar.Date `bun:"start_date,notnull" json:"start_date"`
	EndDate           calendar.Date `bun:"end_date,notnull" json:"end_date"`
	Reason            string        `bun:"reason" json:"reason,omitempty"`

	// Relations not stored in the database
	Group           *Group             `bun:"-" json:"group,omitempty"`
	RegularStaff    *SubstitutionStaff `bun:"-" json:"regular_staff,omitempty"`
	SubstituteStaff *SubstitutionStaff `bun:"-" json:"substitute_staff,omitempty"`
}

// SubstitutionStaff is a staff member a substitution names, as School
// Membership resolves it. Person stays nil until the People Directory
// resolved the name.
type SubstitutionStaff struct {
	ID       int64               `json:"id"`
	PersonID int64               `json:"person_id"`
	Person   *SubstitutionPerson `json:"person,omitempty"`
}

// SubstitutionPerson is the name of a substitution's staff member.
type SubstitutionPerson struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// FullName returns "Vorname Nachname".
func (p *SubstitutionPerson) FullName() string {
	return p.FirstName + " " + p.LastName
}

// Validate ensures group substitution data is valid
func (gs *GroupSubstitution) Validate() error {
	if gs.GroupID <= 0 {
		return errors.New("group ID is required")
	}

	// RegularStaffID is now optional - only validate if provided
	if gs.RegularStaffID != nil && *gs.RegularStaffID <= 0 {
		return errors.New("regular staff ID must be positive if provided")
	}

	if gs.SubstituteStaffID <= 0 {
		return errors.New("substitute staff ID is required")
	}

	if gs.StartDate.IsZero() {
		return errors.New("start date is required")
	}

	if gs.EndDate.IsZero() {
		return errors.New("end date is required")
	}

	if gs.EndDate.Before(gs.StartDate) {
		return errors.New("end date cannot be before start date")
	}

	// Check that regular and substitute staff are not the same (only if regular staff is specified)
	if gs.RegularStaffID != nil && *gs.RegularStaffID == gs.SubstituteStaffID {
		return errors.New("regular staff and substitute staff cannot be the same")
	}

	return nil
}

// Duration returns the duration of the substitution in days
func (gs *GroupSubstitution) Duration() int {
	return gs.StartDate.DaysUntil(gs.EndDate) + 1
}

// SetGroup links this substitution to a group
func (gs *GroupSubstitution) SetGroup(group *Group) {
	gs.Group = group
	if group != nil {
		gs.GroupID = group.ID
	}
}
