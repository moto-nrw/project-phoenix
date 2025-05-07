package education

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/uptrace/bun"
)

// CombinedGroupTeacherRepository implements education.CombinedGroupTeacherRepository
type CombinedGroupTeacherRepository struct {
	db *bun.DB
}

// NewCombinedGroupTeacherRepository creates a new combined group teacher repository
func NewCombinedGroupTeacherRepository(db *bun.DB) education.CombinedGroupTeacherRepository {
	return &CombinedGroupTeacherRepository{db: db}
}

// Create inserts a new combined group teacher into the database
func (r *CombinedGroupTeacherRepository) Create(ctx context.Context, cgt *education.CombinedGroupTeacher) error {
	if err := cgt.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(cgt).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a combined group teacher by its ID
func (r *CombinedGroupTeacherRepository) FindByID(ctx context.Context, id interface{}) (*education.CombinedGroupTeacher, error) {
	cgt := new(education.CombinedGroupTeacher)
	err := r.db.NewSelect().Model(cgt).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return cgt, nil
}

// FindByCombinedGroup retrieves all teachers of a combined group
func (r *CombinedGroupTeacherRepository) FindByCombinedGroup(ctx context.Context, combinedGroupID int64) ([]*education.CombinedGroupTeacher, error) {
	var teachers []*education.CombinedGroupTeacher
	err := r.db.NewSelect().
		Model(&teachers).
		Where("combined_group_id = ?", combinedGroupID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_combined_group", Err: err}
	}
	return teachers, nil
}

// FindByTeacher retrieves all combined group assignments for a teacher
func (r *CombinedGroupTeacherRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*education.CombinedGroupTeacher, error) {
	var teachers []*education.CombinedGroupTeacher
	err := r.db.NewSelect().
		Model(&teachers).
		Where("teacher_id = ?", teacherID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_teacher", Err: err}
	}
	return teachers, nil
}

// FindByCombinedGroupAndTeacher retrieves a combined group teacher by combined group and teacher
func (r *CombinedGroupTeacherRepository) FindByCombinedGroupAndTeacher(ctx context.Context, combinedGroupID, teacherID int64) (*education.CombinedGroupTeacher, error) {
	teacher := new(education.CombinedGroupTeacher)
	err := r.db.NewSelect().
		Model(teacher).
		Where("combined_group_id = ?", combinedGroupID).
		Where("teacher_id = ?", teacherID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_combined_group_and_teacher", Err: err}
	}
	return teacher, nil
}

// DeleteByCombinedGroup deletes all teachers of a combined group
func (r *CombinedGroupTeacherRepository) DeleteByCombinedGroup(ctx context.Context, combinedGroupID int64) error {
	_, err := r.db.NewDelete().
		Model((*education.CombinedGroupTeacher)(nil)).
		Where("combined_group_id = ?", combinedGroupID).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "delete_by_combined_group", Err: err}
	}
	return nil
}

// DeleteByTeacher deletes all combined group assignments for a teacher
func (r *CombinedGroupTeacherRepository) DeleteByTeacher(ctx context.Context, teacherID int64) error {
	_, err := r.db.NewDelete().
		Model((*education.CombinedGroupTeacher)(nil)).
		Where("teacher_id = ?", teacherID).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "delete_by_teacher", Err: err}
	}
	return nil
}

// Update updates an existing combined group teacher
func (r *CombinedGroupTeacherRepository) Update(ctx context.Context, cgt *education.CombinedGroupTeacher) error {
	if err := cgt.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(cgt).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a combined group teacher
func (r *CombinedGroupTeacherRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*education.CombinedGroupTeacher)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves combined group teachers matching the filters
func (r *CombinedGroupTeacherRepository) List(ctx context.Context, filters map[string]interface{}) ([]*education.CombinedGroupTeacher, error) {
	var teachers []*education.CombinedGroupTeacher
	query := r.db.NewSelect().Model(&teachers)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return teachers, nil
}
