package education

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// GroupTeacher represents the many-to-many relationship between groups and teachers
type GroupTeacher struct {
	base.Model `bun:"schema:education,table:group_teacher"`
	GroupID    int64 `bun:"group_id,notnull" json:"group_id"`
	TeacherID  int64 `bun:"teacher_id,notnull" json:"teacher_id"`

	// Relations not stored in the database
	Group   *Group         `bun:"-" json:"group,omitempty"`
	Teacher *users.Teacher `bun:"-" json:"teacher,omitempty"`
}

func (gt *GroupTeacher) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr("education.group_teacher")
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr("education.group_teacher")
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr("education.group_teacher")
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr("education.group_teacher")
	}
	return nil
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

// GetID returns the entity's ID
func (m *GroupTeacher) GetID() interface{} {
	return m.ID
}

// GetCreatedAt returns the creation timestamp
func (m *GroupTeacher) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (m *GroupTeacher) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}
