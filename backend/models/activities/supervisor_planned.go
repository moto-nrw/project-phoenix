package activities

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// SupervisorPlanned represents a staff member assigned to supervise an activity group
type SupervisorPlanned struct {
	base.Model `bun:"schema:activities,table:supervisors"`
	StaffID    int64 `bun:"staff_id,notnull" json:"staff_id"`
	GroupID    int64 `bun:"group_id,notnull" json:"group_id"`
	IsPrimary  bool  `bun:"is_primary,notnull,default:false" json:"is_primary"`

	// Relations - these would be populated when using the ORM's relations
	// Staff *users.Staff `bun:"rel:belongs-to,join:staff_id=id" json:"staff,omitempty"`
	// Group *Group       `bun:"rel:belongs-to,join:group_id=id" json:"group,omitempty"`
}

func (sp *SupervisorPlanned) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr("activities.supervisors")
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr("activities.supervisors")
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr("activities.supervisors")
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr("activities.supervisors")
	}
	return nil
}

// GetID returns the entity's ID
func (sp *SupervisorPlanned) GetID() interface{} {
	return sp.ID
}

// GetCreatedAt returns the creation timestamp
func (sp *SupervisorPlanned) GetCreatedAt() time.Time {
	return sp.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (sp *SupervisorPlanned) GetUpdatedAt() time.Time {
	return sp.UpdatedAt
}

// TableName returns the database table name
func (sp *SupervisorPlanned) TableName() string {
	return "activities.supervisors"
}

// Validate ensures supervisor planned data is valid
func (sp *SupervisorPlanned) Validate() error {
	if sp.StaffID <= 0 {
		return errors.New("staff ID is required")
	}

	if sp.GroupID <= 0 {
		return errors.New("group ID is required")
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
