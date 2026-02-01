package active

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// GroupSupervisor represents a staff member assigned to supervise an active group
type GroupSupervisor struct {
	base.Model `bun:"schema:active,table:group_supervisors"`
	StaffID    int64      `bun:"staff_id,notnull" json:"staff_id"`
	GroupID    int64      `bun:"group_id,notnull" json:"group_id"`
	Role       string     `bun:"role,notnull" json:"role"`
	StartDate  time.Time  `bun:"start_date,notnull" json:"start_date"`
	EndDate    *time.Time `bun:"end_date" json:"end_date,omitempty"`

	// Relations - these would be populated when using the ORM's relations
	Staff       *users.Staff `bun:"rel:belongs-to,join:staff_id=id" json:"staff,omitempty"`
	ActiveGroup *Group       `bun:"rel:belongs-to,join:group_id=id" json:"active_group,omitempty"`
}

// Table name constants for BUN ORM schema qualification
const (
	tableGroupSupervisors       = "active.group_supervisors"
	tableExprGroupSupervisorsAs = `active.group_supervisors AS "group_supervisor"`
)

// BeforeAppendModel is commented out to let the repository control the table expression
// (This follows the same pattern as active.Group model to avoid potential BUN ORM conflicts)
// func (gs *GroupSupervisor) BeforeAppendModel(query any) error {
// 	if q, ok := query.(*bun.SelectQuery); ok {
// 		q.ModelTableExpr(tableExprGroupSupervisorsAs)
// 	}
// 	if q, ok := query.(*bun.InsertQuery); ok {
// 		q.ModelTableExpr(tableGroupSupervisors)
// 	}
// 	if q, ok := query.(*bun.UpdateQuery); ok {
// 		q.ModelTableExpr(tableGroupSupervisors)
// 	}
// 	if q, ok := query.(*bun.DeleteQuery); ok {
// 		q.ModelTableExpr(tableGroupSupervisors)
// 	}
// 	return nil
// }

// GetID returns the entity's ID
func (gs *GroupSupervisor) GetID() interface{} {
	return gs.ID
}

// GetCreatedAt returns the creation timestamp
func (gs *GroupSupervisor) GetCreatedAt() time.Time {
	return gs.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (gs *GroupSupervisor) GetUpdatedAt() time.Time {
	return gs.UpdatedAt
}

// TableName returns the database table name
func (gs *GroupSupervisor) TableName() string {
	return "active.group_supervisors"
}

// Validate ensures group supervisor data is valid
func (gs *GroupSupervisor) Validate() error {
	if gs.StaffID <= 0 {
		return errors.New("staff ID is required")
	}

	if gs.GroupID <= 0 {
		return errors.New("group ID is required")
	}

	if gs.Role == "" {
		return errors.New("role is required")
	}

	if gs.StartDate.IsZero() {
		return errors.New("start date is required")
	}

	if gs.EndDate != nil && gs.StartDate.After(*gs.EndDate) {
		return errors.New("start date must be before end date")
	}

	return nil
}

// IsActive returns whether this supervision is currently active
func (gs *GroupSupervisor) IsActive() bool {
	if gs.EndDate == nil {
		return true
	}
	return time.Now().Before(*gs.EndDate)
}

// EndSupervision sets the end date to the current date
func (gs *GroupSupervisor) EndSupervision() {
	now := time.Now()
	gs.EndDate = &now
}

// SetEndDate explicitly sets the end date
func (gs *GroupSupervisor) SetEndDate(endDate time.Time) error {
	if gs.StartDate.After(endDate) {
		return errors.New("end date cannot be before start date")
	}
	gs.EndDate = &endDate
	return nil
}
