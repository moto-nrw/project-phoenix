// backend/database/repositories/education/group_substitution.go
package education

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/uptrace/bun"
)

// GroupSubstitutionRepository implements education.GroupSubstitutionRepository interface
type GroupSubstitutionRepository struct {
	*base.Repository[*education.GroupSubstitution]
	db *bun.DB
}

// NewGroupSubstitutionRepository creates a new GroupSubstitutionRepository
func NewGroupSubstitutionRepository(db *bun.DB) education.GroupSubstitutionRepository {
	return &GroupSubstitutionRepository{
		Repository: base.NewRepository[*education.GroupSubstitution](db, "education.group_substitution", "GroupSubstitution"),
		db:         db,
	}
}

// FindByGroup retrieves all substitutions for a specific group
func (r *GroupSubstitutionRepository) FindByGroup(ctx context.Context, groupID int64) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	err := r.db.NewSelect().
		Model(&substitutions).
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
		Where("start_date <= ? AND end_date >= ?", date, date).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active",
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
	// Convert old filter format to new QueryOptions
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			switch field {
			case "active":
				if boolValue, ok := value.(bool); ok && boolValue {
					now := time.Now()
					filter.DateBetween("start_date", "end_date", now)
				}
			case "date":
				if dateValue, ok := value.(time.Time); ok {
					filter.DateBetween("start_date", "end_date", dateValue)
				}
			case "reason_like":
				if strValue, ok := value.(string); ok {
					filter.ILike("reason", "%"+strValue+"%")
				}
			default:
				// Default to exact match for other fields
				filter.Equal(field, value)
			}
		}
	}

	options.Filter = filter

	return r.ListWithOptions(ctx, options)
}

// ListWithOptions provides a type-safe way to list group substitutions with query options
func (r *GroupSubstitutionRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	query := r.db.NewSelect().Model(&substitutions)

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
