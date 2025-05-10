package education

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// GroupTeacher represents the many-to-many relationship between groups and teachers
type GroupTeacher struct {
	base.Model
	GroupID   int64 `bun:"group_id,notnull" json:"group_id"`
	TeacherID int64 `bun:"teacher_id,notnull" json:"teacher_id"`

	// Relations not stored in the database
	Group   *Group         `bun:"-" json:"group,omitempty"`
	Teacher *users.Teacher `bun:"-" json:"teacher,omitempty"`
}

// TableName returns the database table name
func (gt *GroupTeacher) TableName() string {
	return "education.group_teacher"
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

// SetTeacher sets the associated teacher
func (gt *GroupTeacher) SetTeacher(teacher *users.Teacher) {
	gt.Teacher = teacher
	if teacher != nil {
		gt.TeacherID = teacher.ID
	}
}
