package education

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// CombinedGroupTeacher represents a teacher assigned to a combined group
type CombinedGroupTeacher struct {
	base.Model
	CombinedGroupID int64 `bun:"combined_group_id,notnull" json:"combined_group_id"`
	TeacherID       int64 `bun:"teacher_id,notnull" json:"teacher_id"`

	// Relations
	CombinedGroup *CombinedGroup `bun:"rel:belongs-to,join:combined_group_id=id" json:"combined_group,omitempty"`
	Teacher       *users.Teacher `bun:"rel:belongs-to,join:teacher_id=id" json:"teacher,omitempty"`
}

// TableName returns the table name for the CombinedGroupTeacher model
func (cgt *CombinedGroupTeacher) TableName() string {
	return "education.combined_group_teacher"
}

// GetID returns the combined group teacher ID
func (cgt *CombinedGroupTeacher) GetID() interface{} {
	return cgt.ID
}

// GetCreatedAt returns the creation timestamp
func (cgt *CombinedGroupTeacher) GetCreatedAt() time.Time {
	return cgt.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (cgt *CombinedGroupTeacher) GetUpdatedAt() time.Time {
	return cgt.CreatedAt // This model only has created_at, no updated_at
}

// Validate validates the combined group teacher fields
func (cgt *CombinedGroupTeacher) Validate() error {
	if cgt.CombinedGroupID <= 0 {
		return errors.New("combined group ID is required")
	}

	if cgt.TeacherID <= 0 {
		return errors.New("teacher ID is required")
	}

	return nil
}

// CombinedGroupTeacherRepository defines operations for working with combined group teachers
type CombinedGroupTeacherRepository interface {
	base.Repository[*CombinedGroupTeacher]
	FindByCombinedGroup(ctx context.Context, combinedGroupID int64) ([]*CombinedGroupTeacher, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*CombinedGroupTeacher, error)
	FindByCombinedGroupAndTeacher(ctx context.Context, combinedGroupID, teacherID int64) (*CombinedGroupTeacher, error)
	DeleteByCombinedGroup(ctx context.Context, combinedGroupID int64) error
	DeleteByTeacher(ctx context.Context, teacherID int64) error
}

// DefaultCombinedGroupTeacherRepository is the default implementation of CombinedGroupTeacherRepository
type DefaultCombinedGroupTeacherRepository struct {
	db *bun.DB
}

// NewCombinedGroupTeacherRepository creates a new combined group teacher repository
func NewCombinedGroupTeacherRepository(db *bun.DB) CombinedGroupTeacherRepository {
	return &DefaultCombinedGroupTeacherRepository{db: db}
}

// Create inserts a new combined group teacher into the database
func (r *DefaultCombinedGroupTeacherRepository) Create(ctx context.Context, combinedGroupTeacher *CombinedGroupTeacher) error {
	if err := combinedGroupTeacher.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(combinedGroupTeacher).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a combined group teacher by its ID
func (r *DefaultCombinedGroupTeacherRepository) FindByID(ctx context.Context, id interface{}) (*CombinedGroupTeacher, error) {
	combinedGroupTeacher := new(CombinedGroupTeacher)
	err := r.db.NewSelect().Model(combinedGroupTeacher).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return combinedGroupTeacher, nil
}

// FindByCombinedGroup retrieves all teachers of a combined group
func (r *DefaultCombinedGroupTeacherRepository) FindByCombinedGroup(ctx context.Context, combinedGroupID int64) ([]*CombinedGroupTeacher, error) {
	var combinedGroupTeachers []*CombinedGroupTeacher
	err := r.db.NewSelect().
		Model(&combinedGroupTeachers).
		Where("combined_group_id = ?", combinedGroupID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_combined_group", Err: err}
	}
	return combinedGroupTeachers, nil
}

// FindByTeacher retrieves all combined group assignments for a teacher
func (r *DefaultCombinedGroupTeacherRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*CombinedGroupTeacher, error) {
	var combinedGroupTeachers []*CombinedGroupTeacher
	err := r.db.NewSelect().
		Model(&combinedGroupTeachers).
		Where("teacher_id = ?", teacherID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_teacher", Err: err}
	}
	return combinedGroupTeachers, nil
}

// FindByCombinedGroupAndTeacher retrieves a combined group teacher by combined group and teacher
func (r *DefaultCombinedGroupTeacherRepository) FindByCombinedGroupAndTeacher(ctx context.Context, combinedGroupID, teacherID int64) (*CombinedGroupTeacher, error) {
	combinedGroupTeacher := new(CombinedGroupTeacher)
	err := r.db.NewSelect().
		Model(combinedGroupTeacher).
		Where("combined_group_id = ?", combinedGroupID).
		Where("teacher_id = ?", teacherID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_combined_group_and_teacher", Err: err}
	}
	return combinedGroupTeacher, nil
}

// DeleteByCombinedGroup deletes all teachers of a combined group
func (r *DefaultCombinedGroupTeacherRepository) DeleteByCombinedGroup(ctx context.Context, combinedGroupID int64) error {
	_, err := r.db.NewDelete().
		Model((*CombinedGroupTeacher)(nil)).
		Where("combined_group_id = ?", combinedGroupID).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "delete_by_combined_group", Err: err}
	}
	return nil
}

// DeleteByTeacher deletes all combined group assignments for a teacher
func (r *DefaultCombinedGroupTeacherRepository) DeleteByTeacher(ctx context.Context, teacherID int64) error {
	_, err := r.db.NewDelete().
		Model((*CombinedGroupTeacher)(nil)).
		Where("teacher_id = ?", teacherID).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "delete_by_teacher", Err: err}
	}
	return nil
}

// Update updates an existing combined group teacher
func (r *DefaultCombinedGroupTeacherRepository) Update(ctx context.Context, combinedGroupTeacher *CombinedGroupTeacher) error {
	if err := combinedGroupTeacher.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(combinedGroupTeacher).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a combined group teacher
func (r *DefaultCombinedGroupTeacherRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*CombinedGroupTeacher)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves combined group teachers matching the filters
func (r *DefaultCombinedGroupTeacherRepository) List(ctx context.Context, filters map[string]interface{}) ([]*CombinedGroupTeacher, error) {
	var combinedGroupTeachers []*CombinedGroupTeacher
	query := r.db.NewSelect().Model(&combinedGroupTeachers)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return combinedGroupTeachers, nil
}
