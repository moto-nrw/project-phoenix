package education

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// GroupSubstitution represents a temporary substitution of a staff member for another in a group
type GroupSubstitution struct {
	base.Model        `bun:"schema:education,table:group_substitution"`
	GroupID           int64     `bun:"group_id,notnull" json:"group_id"`
	RegularStaffID    int64     `bun:"regular_staff_id,notnull" json:"regular_staff_id"`
	SubstituteStaffID int64     `bun:"substitute_staff_id,notnull" json:"substitute_staff_id"`
	StartDate         time.Time `bun:"start_date,notnull" json:"start_date"`
	EndDate           time.Time `bun:"end_date,notnull" json:"end_date"`
	Reason            string    `bun:"reason" json:"reason,omitempty"`

	// Relations not stored in the database
	Group           *Group       `bun:"-" json:"group,omitempty"`
	RegularStaff    *users.Staff `bun:"-" json:"regular_staff,omitempty"`
	SubstituteStaff *users.Staff `bun:"-" json:"substitute_staff,omitempty"`
}

func (gs *GroupSubstitution) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr("education.group_substitution")
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr("education.group_substitution")
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr("education.group_substitution")
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr("education.group_substitution")
	}
	return nil
}

// TableName returns the database table name
func (gs *GroupSubstitution) TableName() string {
	return "education.group_substitution"
}

// Validate ensures group substitution data is valid
func (gs *GroupSubstitution) Validate() error {
	if gs.GroupID <= 0 {
		return errors.New("group ID is required")
	}

	if gs.RegularStaffID <= 0 {
		return errors.New("regular staff ID is required")
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

	// Check that regular and substitute staff are not the same
	if gs.RegularStaffID == gs.SubstituteStaffID {
		return errors.New("regular staff and substitute staff cannot be the same")
	}

	return nil
}

// Duration returns the duration of the substitution in days
func (gs *GroupSubstitution) Duration() int {
	return int(gs.EndDate.Sub(gs.StartDate).Hours()/24) + 1
}

// IsActive checks if the substitution is currently active
func (gs *GroupSubstitution) IsActive(checkDate time.Time) bool {
	return !checkDate.Before(gs.StartDate) && !checkDate.After(gs.EndDate)
}

// IsCurrentlyActive checks if the substitution is active at the current time
func (gs *GroupSubstitution) IsCurrentlyActive() bool {
	return gs.IsActive(time.Now())
}

// SetGroup links this substitution to a group
func (gs *GroupSubstitution) SetGroup(group *Group) {
	gs.Group = group
	if group != nil {
		gs.GroupID = group.ID
	}
}

// SetRegularStaff links this substitution to the regular staff member
func (gs *GroupSubstitution) SetRegularStaff(staff *users.Staff) {
	gs.RegularStaff = staff
	if staff != nil {
		gs.RegularStaffID = staff.ID
	}
}

// SetSubstituteStaff links this substitution to the substitute staff member
func (gs *GroupSubstitution) SetSubstituteStaff(staff *users.Staff) {
	gs.SubstituteStaff = staff
	if staff != nil {
		gs.SubstituteStaffID = staff.ID
	}
}

// GetID returns the entity's ID
func (m *GroupSubstitution) GetID() interface{} {
	return m.ID
}

// GetCreatedAt returns the creation timestamp
func (m *GroupSubstitution) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (m *GroupSubstitution) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}
