package education

import (
	"errors"
)

// GroupTeacher represents the many-to-many relationship between groups and teachers
type GroupTeacher struct {
	Model
	TenantModel
	GroupID   int64 `bun:"group_id,notnull" json:"group_id"`
	TeacherID int64 `bun:"teacher_id,notnull" json:"teacher_id"`

	// Relations not stored in the database
	Group *Group `bun:"-" json:"group,omitempty"`
}

// Validate ensures group teacher data is valid
func (gt *GroupTeacher) Validate() error {
	if gt.GroupID <= 0 {
		return errors.New("group ID is required")
	}

	if gt.TeacherID <= 0 {
		return errors.New("teacher ID is required")
	}

	return nil
}

// SetGroup sets the associated group
func (gt *GroupTeacher) SetGroup(group *Group) {
	gt.Group = group
	if group != nil {
		gt.GroupID = group.ID
	}
}
