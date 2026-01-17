package education

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	"github.com/uptrace/bun"
)

// Table name constant to avoid string literal duplication
const tableGroupSubstitution = "education.group_substitution"

// Query constants (S1192 - avoid duplicate string literals)
const dateRangeContainsCondition = "start_date <= ? AND end_date >= ?"

// GroupSubstitutionRepository implements education.GroupSubstitutionRepository and
// education.GroupSubstitutionRelationsRepository.
type GroupSubstitutionRepository struct {
	*base.Repository[*education.GroupSubstitution]
	db *bun.DB
}

// NewGroupSubstitutionRepository creates a new GroupSubstitutionRepository
func NewGroupSubstitutionRepository(db *bun.DB) *GroupSubstitutionRepository {
	return &GroupSubstitutionRepository{
		Repository: base.NewRepository[*education.GroupSubstitution](db, tableGroupSubstitution, "group_substitution"),
		db:         db,
	}
}

// FindByGroup retrieves all substitutions for a specific group
func (r *GroupSubstitutionRepository) FindByGroup(ctx context.Context, groupID int64) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	err := r.db.NewSelect().
		Model(&substitutions).
		ModelTableExpr(tableGroupSubstitution).
		Where("group_id = ?", groupID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group",
			Err: err,
		}
	}

	return substitutions, nil
}

// FindByRegularStaff retrieves all substitutions for a regular staff member
func (r *GroupSubstitutionRepository) FindByRegularStaff(ctx context.Context, staffID int64) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	err := r.db.NewSelect().
		Model(&substitutions).
		ModelTableExpr(tableGroupSubstitution).
		Where("regular_staff_id = ?", staffID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by regular staff",
			Err: err,
		}
	}

	return substitutions, nil
}

// FindBySubstituteStaff retrieves all substitutions where a staff member is substituting
func (r *GroupSubstitutionRepository) FindBySubstituteStaff(ctx context.Context, staffID int64) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	err := r.db.NewSelect().
		Model(&substitutions).
		ModelTableExpr(tableGroupSubstitution).
		Where("substitute_staff_id = ?", staffID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by substitute staff",
			Err: err,
		}
	}

	return substitutions, nil
}

// FindActive retrieves all active substitutions for a specific date
func (r *GroupSubstitutionRepository) FindActive(ctx context.Context, date time.Time) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	err := r.db.NewSelect().
		Model(&substitutions).
		ModelTableExpr(tableGroupSubstitution).
		Where(dateRangeContainsCondition, date, date).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active",
			Err: err,
		}
	}

	return substitutions, nil
}

// FindActiveBySubstitute retrieves all active substitutions for a staff member and date
func (r *GroupSubstitutionRepository) FindActiveBySubstitute(ctx context.Context, substituteStaffID int64, date time.Time) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	err := r.db.NewSelect().
		Model(&substitutions).
		ModelTableExpr(tableGroupSubstitution).
		Where("substitute_staff_id = ?", substituteStaffID).
		Where(dateRangeContainsCondition, date, date).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by substitute",
			Err: err,
		}
	}

	return substitutions, nil
}

// FindActiveByGroup retrieves all active substitutions for a specific group and date
func (r *GroupSubstitutionRepository) FindActiveByGroup(ctx context.Context, groupID int64, date time.Time) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	err := r.db.NewSelect().
		Model(&substitutions).
		ModelTableExpr(tableGroupSubstitution).
		Where("group_id = ? AND start_date <= ? AND end_date >= ?", groupID, date, date).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by group",
			Err: err,
		}
	}

	return substitutions, nil
}

// FindOverlapping finds all substitutions that overlap with the given date range for a staff member
func (r *GroupSubstitutionRepository) FindOverlapping(ctx context.Context, staffID int64, startDate time.Time, endDate time.Time) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	err := r.db.NewSelect().
		Model(&substitutions).
		ModelTableExpr(tableGroupSubstitution).
		Where("(regular_staff_id = ? OR substitute_staff_id = ?)", staffID, staffID).
		Where("start_date <= ? AND end_date >= ?", endDate, startDate).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find overlapping",
			Err: err,
		}
	}

	return substitutions, nil
}

// Create overrides the base Create method to handle validation
func (r *GroupSubstitutionRepository) Create(ctx context.Context, substitution *education.GroupSubstitution) error {
	if substitution == nil {
		return fmt.Errorf("group substitution cannot be nil")
	}

	// Validate group substitution
	if err := substitution.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, substitution)
}

// Update overrides the base Update method to handle validation
func (r *GroupSubstitutionRepository) Update(ctx context.Context, substitution *education.GroupSubstitution) error {
	if substitution == nil {
		return fmt.Errorf("group substitution cannot be nil")
	}

	// Validate group substitution
	if err := substitution.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, substitution)
}

// List retrieves group substitutions matching the provided filters
func (r *GroupSubstitutionRepository) List(ctx context.Context, filters map[string]interface{}) ([]*education.GroupSubstitution, error) {
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			applySubstitutionFilter(filter, field, value)
		}
	}

	options.Filter = filter
	return r.ListWithOptions(ctx, options)
}

// ListWithOptions provides a type-safe way to list group substitutions with query options
func (r *GroupSubstitutionRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	query := r.db.NewSelect().
		Model(&substitutions).
		ModelTableExpr(tableGroupSubstitution)

	// Apply query options
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list with options",
			Err: err,
		}
	}

	return substitutions, nil
}
