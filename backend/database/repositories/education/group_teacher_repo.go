package education

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/uptrace/bun"
)

// GroupTeacherRepository implements education.GroupTeacherRepository
type GroupTeacherRepository struct {
	db *bun.DB
}

// NewGroupTeacherRepository creates a new group teacher repository
func NewGroupTeacherRepository(db *bun.DB) education.GroupTeacherRepository {
	return &GroupTeacherRepository{db: db}
}

// Create inserts a new group teacher into the database
func (r *GroupTeacherRepository) Create(ctx context.Context, gt *education.GroupTeacher) error {
	if err := gt.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(gt).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a group teacher by its ID
func (r *GroupTeacherRepository) FindByID(ctx context.Context, id interface{}) (*education.GroupTeacher, error) {
	gt := new(education.GroupTeacher)
	err := r.db.NewSelect().Model(gt).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return gt, nil
}

// FindByGroup retrieves all group teachers for a group
func (r *GroupTeacherRepository) FindByGroup(ctx context.Context, groupID int64) ([]*education.GroupTeacher, error) {
	var groupTeachers []*education.GroupTeacher
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
func (r *GroupTeacherRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*education.GroupTeacher, error) {
	var groupTeachers []*education.GroupTeacher
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
func (r *GroupTeacherRepository) FindByGroupAndTeacher(ctx context.Context, groupID, teacherID int64) (*education.GroupTeacher, error) {
	groupTeacher := new(education.GroupTeacher)
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
func (r *GroupTeacherRepository) DeleteByGroup(ctx context.Context, groupID int64) error {
	_, err := r.db.NewDelete().
		Model((*education.GroupTeacher)(nil)).
		Where("group_id = ?", groupID).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "delete_by_group", Err: err}
	}
	return nil
}

// DeleteByTeacher deletes all group teachers for a teacher
func (r *GroupTeacherRepository) DeleteByTeacher(ctx context.Context, teacherID int64) error {
	_, err := r.db.NewDelete().
		Model((*education.GroupTeacher)(nil)).
		Where("teacher_id = ?", teacherID).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "delete_by_teacher", Err: err}
	}
	return nil
}

// Update updates an existing group teacher
func (r *GroupTeacherRepository) Update(ctx context.Context, gt *education.GroupTeacher) error {
	if err := gt.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(gt).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a group teacher
func (r *GroupTeacherRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*education.GroupTeacher)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves group teachers matching the filters
func (r *GroupTeacherRepository) List(ctx context.Context, filters map[string]interface{}) ([]*education.GroupTeacher, error) {
	var groupTeachers []*education.GroupTeacher
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
