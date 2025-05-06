package education

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// GroupTeacher represents a teacher assigned to an education group
type GroupTeacher struct {
	base.Model
	GroupID   int64 `bun:"group_id,notnull" json:"group_id"`
	TeacherID int64 `bun:"teacher_id,notnull" json:"teacher_id"`

	// Relations
	Group   *Group         `bun:"rel:belongs-to,join:group_id=id" json:"group,omitempty"`
	Teacher *users.Teacher `bun:"rel:belongs-to,join:teacher_id=id" json:"teacher,omitempty"`
}

// TableName returns the table name for the GroupTeacher model
func (gt *GroupTeacher) TableName() string {
	return "education.group_teacher"
}

// GetID returns the group teacher ID
func (gt *GroupTeacher) GetID() interface{} {
	return gt.ID
}

// GetCreatedAt returns the creation timestamp
func (gt *GroupTeacher) GetCreatedAt() time.Time {
	return gt.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (gt *GroupTeacher) GetUpdatedAt() time.Time {
	return gt.UpdatedAt
}

// Validate validates the group teacher fields
func (gt *GroupTeacher) Validate() error {
	if gt.GroupID <= 0 {
		return errors.New("group ID is required")
	}

	if gt.TeacherID <= 0 {
		return errors.New("teacher ID is required")
	}

	return nil
}

// GroupTeacherRepository defines operations for working with group teachers
type GroupTeacherRepository interface {
	base.Repository[*GroupTeacher]
	FindByGroup(ctx context.Context, groupID int64) ([]*GroupTeacher, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*GroupTeacher, error)
	DeleteByGroup(ctx context.Context, groupID int64) error
	DeleteByTeacher(ctx context.Context, teacherID int64) error
	FindByGroupAndTeacher(ctx context.Context, groupID, teacherID int64) (*GroupTeacher, error)
}

// DefaultGroupTeacherRepository is the default implementation of GroupTeacherRepository
type DefaultGroupTeacherRepository struct {
	db *bun.DB
}

// NewGroupTeacherRepository creates a new group teacher repository
func NewGroupTeacherRepository(db *bun.DB) GroupTeacherRepository {
	return &DefaultGroupTeacherRepository{db: db}
}

// Create inserts a new group teacher into the database
func (r *DefaultGroupTeacherRepository) Create(ctx context.Context, groupTeacher *GroupTeacher) error {
	if err := groupTeacher.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(groupTeacher).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a group teacher by its ID
func (r *DefaultGroupTeacherRepository) FindByID(ctx context.Context, id interface{}) (*GroupTeacher, error) {
	groupTeacher := new(GroupTeacher)
	err := r.db.NewSelect().Model(groupTeacher).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return groupTeacher, nil
}

// FindByGroup retrieves all group teachers for a group
func (r *DefaultGroupTeacherRepository) FindByGroup(ctx context.Context, groupID int64) ([]*GroupTeacher, error) {
	var groupTeachers []*GroupTeacher
	err := r.db.NewSelect().
		Model(&groupTeachers).
		Where("group_id = ?", groupID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_group", Err: err}
	}
	return groupTeachers, nil
}

// FindByTeacher retrieves all group teachers for a teacher
func (r *DefaultGroupTeacherRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*GroupTeacher, error) {
	var groupTeachers []*GroupTeacher
	err := r.db.NewSelect().
		Model(&groupTeachers).
		Where("teacher_id = ?", teacherID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_teacher", Err: err}
	}
	return groupTeachers, nil
}

// FindByGroupAndTeacher retrieves a group teacher by group and teacher
func (r *DefaultGroupTeacherRepository) FindByGroupAndTeacher(ctx context.Context, groupID, teacherID int64) (*GroupTeacher, error) {
	groupTeacher := new(GroupTeacher)
	err := r.db.NewSelect().
		Model(groupTeacher).
		Where("group_id = ?", groupID).
		Where("teacher_id = ?", teacherID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_group_and_teacher", Err: err}
	}
	return groupTeacher, nil
}

// DeleteByGroup deletes all group teachers for a group
func (r *DefaultGroupTeacherRepository) DeleteByGroup(ctx context.Context, groupID int64) error {
	_, err := r.db.NewDelete().
		Model((*GroupTeacher)(nil)).
		Where("group_id = ?", groupID).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "delete_by_group", Err: err}
	}
	return nil
}

// DeleteByTeacher deletes all group teachers for a teacher
func (r *DefaultGroupTeacherRepository) DeleteByTeacher(ctx context.Context, teacherID int64) error {
	_, err := r.db.NewDelete().
		Model((*GroupTeacher)(nil)).
		Where("teacher_id = ?", teacherID).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "delete_by_teacher", Err: err}
	}
	return nil
}

// Update updates an existing group teacher
func (r *DefaultGroupTeacherRepository) Update(ctx context.Context, groupTeacher *GroupTeacher) error {
	if err := groupTeacher.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(groupTeacher).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a group teacher
func (r *DefaultGroupTeacherRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*GroupTeacher)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves group teachers matching the filters
func (r *DefaultGroupTeacherRepository) List(ctx context.Context, filters map[string]interface{}) ([]*GroupTeacher, error) {
	var groupTeachers []*GroupTeacher
	query := r.db.NewSelect().Model(&groupTeachers)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return groupTeachers, nil
}
