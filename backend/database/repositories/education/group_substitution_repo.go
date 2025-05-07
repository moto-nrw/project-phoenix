package education

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/uptrace/bun"
)

// GroupSubstitutionRepository implements education.GroupSubstitutionRepository
type GroupSubstitutionRepository struct {
	db *bun.DB
}

// NewGroupSubstitutionRepository creates a new group substitution repository
func NewGroupSubstitutionRepository(db *bun.DB) education.GroupSubstitutionRepository {
	return &GroupSubstitutionRepository{db: db}
}

// Create inserts a new group substitution into the database
func (r *GroupSubstitutionRepository) Create(ctx context.Context, gs *education.GroupSubstitution) error {
	if err := gs.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(gs).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a group substitution by its ID
func (r *GroupSubstitutionRepository) FindByID(ctx context.Context, id interface{}) (*education.GroupSubstitution, error) {
	gs := new(education.GroupSubstitution)
	err := r.db.NewSelect().Model(gs).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return gs, nil
}

// FindByGroup retrieves all substitutions for a group
func (r *GroupSubstitutionRepository) FindByGroup(ctx context.Context, groupID int64) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	err := r.db.NewSelect().
		Model(&substitutions).
		Where("group_id = ?", groupID).
		Order("start_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_group", Err: err}
	}
	return substitutions, nil
}

// FindByRegularTeacher retrieves all substitutions for a regular teacher
func (r *GroupSubstitutionRepository) FindByRegularTeacher(ctx context.Context, teacherID int64) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	err := r.db.NewSelect().
		Model(&substitutions).
		Where("regular_teacher_id = ?", teacherID).
		Order("start_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_regular_teacher", Err: err}
	}
	return substitutions, nil
}

// FindBySubstituteTeacher retrieves all substitutions for a substitute teacher
func (r *GroupSubstitutionRepository) FindBySubstituteTeacher(ctx context.Context, teacherID int64) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	err := r.db.NewSelect().
		Model(&substitutions).
		Where("substitute_teacher_id = ?", teacherID).
		Order("start_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_substitute_teacher", Err: err}
	}
	return substitutions, nil
}

// FindActive retrieves all currently active substitutions
func (r *GroupSubstitutionRepository) FindActive(ctx context.Context) ([]*education.GroupSubstitution, error) {
	now := time.Now()
	var substitutions []*education.GroupSubstitution
	err := r.db.NewSelect().
		Model(&substitutions).
		Where("start_date <= ?", now).
		Where("end_date >= ?", now).
		Order("start_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active", Err: err}
	}
	return substitutions, nil
}

// FindActiveByGroup retrieves all currently active substitutions for a group
func (r *GroupSubstitutionRepository) FindActiveByGroup(ctx context.Context, groupID int64) ([]*education.GroupSubstitution, error) {
	now := time.Now()
	var substitutions []*education.GroupSubstitution
	err := r.db.NewSelect().
		Model(&substitutions).
		Where("group_id = ?", groupID).
		Where("start_date <= ?", now).
		Where("end_date >= ?", now).
		Order("start_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active_by_group", Err: err}
	}
	return substitutions, nil
}

// FindByDateRange retrieves all substitutions within a date range
func (r *GroupSubstitutionRepository) FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	err := r.db.NewSelect().
		Model(&substitutions).
		Where("(start_date BETWEEN ? AND ?) OR (end_date BETWEEN ? AND ?) OR (start_date <= ? AND end_date >= ?)",
			startDate, endDate, startDate, endDate, startDate, endDate).
		Order("start_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_date_range", Err: err}
	}
	return substitutions, nil
}

// Update updates an existing group substitution
func (r *GroupSubstitutionRepository) Update(ctx context.Context, gs *education.GroupSubstitution) error {
	if err := gs.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(gs).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a group substitution
func (r *GroupSubstitutionRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*education.GroupSubstitution)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves group substitutions matching the filters
func (r *GroupSubstitutionRepository) List(ctx context.Context, filters map[string]interface{}) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	query := r.db.NewSelect().Model(&substitutions)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return substitutions, nil
}
