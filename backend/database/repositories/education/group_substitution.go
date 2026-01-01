// backend/database/repositories/education/group_substitution.go
package education

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Table name constant to avoid string literal duplication
const tableGroupSubstitution = "education.group_substitution"

// GroupSubstitutionRepository implements education.GroupSubstitutionRepository interface
type GroupSubstitutionRepository struct {
	*base.Repository[*education.GroupSubstitution]
	db *bun.DB
}

// NewGroupSubstitutionRepository creates a new GroupSubstitutionRepository
func NewGroupSubstitutionRepository(db *bun.DB) education.GroupSubstitutionRepository {
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

// FindActiveBySubstitute retrieves all active substitutions for a staff member and date
func (r *GroupSubstitutionRepository) FindActiveBySubstitute(ctx context.Context, substituteStaffID int64, date time.Time) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	err := r.db.NewSelect().
		Model(&substitutions).
		ModelTableExpr(tableGroupSubstitution).
		Where("substitute_staff_id = ?", substituteStaffID).
		Where("start_date <= ? AND end_date >= ?", date, date).
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

// FindByIDWithRelations retrieves a substitution by ID with all related data loaded
func (r *GroupSubstitutionRepository) FindByIDWithRelations(ctx context.Context, id int64) (*education.GroupSubstitution, error) {
	var substitution education.GroupSubstitution

	err := r.db.NewSelect().
		Model(&substitution).
		ModelTableExpr(tableGroupSubstitution).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by id with relations",
			Err: err,
		}
	}

	// Load group
	if substitution.GroupID > 0 {
		var group education.Group
		err = r.db.NewSelect().
			Model(&group).
			ModelTableExpr(`education.groups AS "group"`).
			Where(`"group".id = ?`, substitution.GroupID).
			Scan(ctx)
		if err == nil {
			substitution.Group = &group
		}
	}

	// Load regular staff with person
	if substitution.RegularStaffID != nil && *substitution.RegularStaffID > 0 {
		type staffWithPerson struct {
			ID       int64         `bun:"staff__id"`
			PersonID int64         `bun:"staff__person_id"`
			Staff    *users.Staff  `bun:"staff"`
			Person   *users.Person `bun:"person"`
		}
		var result staffWithPerson

		err = r.db.NewSelect().
			Model(&result).
			ModelTableExpr(`users.staff AS "staff"`).
			ColumnExpr(`"staff".id AS "staff__id"`).
			ColumnExpr(`"staff".person_id AS "staff__person_id"`).
			ColumnExpr(`"staff".* AS "staff__*"`).
			ColumnExpr(`"person".* AS "person__*"`).
			Join(`INNER JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
			Where(`"staff".id = ?`, substitution.RegularStaffID).
			Scan(ctx)

		if err == nil && result.Staff != nil {
			result.Staff.Person = result.Person
			substitution.RegularStaff = result.Staff
		}
	}

	// Load substitute staff with person
	if substitution.SubstituteStaffID > 0 {
		type staffWithPerson struct {
			ID       int64         `bun:"staff__id"`
			PersonID int64         `bun:"staff__person_id"`
			Staff    *users.Staff  `bun:"staff"`
			Person   *users.Person `bun:"person"`
		}
		var result staffWithPerson

		err = r.db.NewSelect().
			Model(&result).
			ModelTableExpr(`users.staff AS "staff"`).
			ColumnExpr(`"staff".id AS "staff__id"`).
			ColumnExpr(`"staff".person_id AS "staff__person_id"`).
			ColumnExpr(`"staff".* AS "staff__*"`).
			ColumnExpr(`"person".* AS "person__*"`).
			Join(`INNER JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
			Where(`"staff".id = ?`, substitution.SubstituteStaffID).
			Scan(ctx)

		if err == nil && result.Staff != nil {
			result.Staff.Person = result.Person
			substitution.SubstituteStaff = result.Staff
		}
	}

	return &substitution, nil
}

// ListWithRelations retrieves substitutions with all related data loaded
func (r *GroupSubstitutionRepository) ListWithRelations(ctx context.Context, options *modelBase.QueryOptions) ([]*education.GroupSubstitution, error) {
	// First get the substitutions
	substitutions, err := r.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}

	// Collect unique IDs
	groupIDs := make(map[int64]bool)
	staffIDs := make(map[int64]bool)

	for _, sub := range substitutions {
		if sub.GroupID > 0 {
			groupIDs[sub.GroupID] = true
		}
		if sub.RegularStaffID != nil && *sub.RegularStaffID > 0 {
			staffIDs[*sub.RegularStaffID] = true
		}
		if sub.SubstituteStaffID > 0 {
			staffIDs[sub.SubstituteStaffID] = true
		}
	}

	// Load all groups at once
	groupMap := make(map[int64]*education.Group)
	if len(groupIDs) > 0 {
		var groups []*education.Group
		groupIDSlice := make([]int64, 0, len(groupIDs))
		for id := range groupIDs {
			groupIDSlice = append(groupIDSlice, id)
		}

		err = r.db.NewSelect().
			Model(&groups).
			ModelTableExpr(`education.groups AS "group"`).
			Where(`"group".id IN (?)`, bun.In(groupIDSlice)).
			Scan(ctx)

		if err == nil {
			for _, group := range groups {
				groupMap[group.ID] = group
			}
		}
	}

	// Load all staff with persons using two separate queries (simpler and more robust than JOINs with BUN)
	staffMap := make(map[int64]*users.Staff)
	if len(staffIDs) > 0 {
		staffIDSlice := make([]int64, 0, len(staffIDs))
		for id := range staffIDs {
			staffIDSlice = append(staffIDSlice, id)
		}

		// Step 1: Load all staff records
		var staffList []*users.Staff
		err = r.db.NewSelect().
			Model(&staffList).
			ModelTableExpr(`users.staff AS "staff"`).
			Where(`"staff".id IN (?)`, bun.In(staffIDSlice)).
			Scan(ctx)

		if err == nil && len(staffList) > 0 {
			// Collect person IDs
			personIDs := make([]int64, 0, len(staffList))
			for _, staff := range staffList {
				if staff.PersonID > 0 {
					personIDs = append(personIDs, staff.PersonID)
				}
				staffMap[staff.ID] = staff
			}

			// Step 2: Load all persons
			if len(personIDs) > 0 {
				var persons []*users.Person
				err = r.db.NewSelect().
					Model(&persons).
					ModelTableExpr(`users.persons AS "person"`).
					Where(`"person".id IN (?)`, bun.In(personIDs)).
					Scan(ctx)

				if err == nil {
					// Create person map
					personMap := make(map[int64]*users.Person)
					for _, person := range persons {
						personMap[person.ID] = person
					}

					// Link persons to staff
					for _, staff := range staffList {
						if person, ok := personMap[staff.PersonID]; ok {
							staff.Person = person
						}
					}
				}
			}
		}
	}

	// Assign loaded data to substitutions
	for _, sub := range substitutions {
		if group, ok := groupMap[sub.GroupID]; ok {
			sub.Group = group
		}
		if sub.RegularStaffID != nil {
			if staff, ok := staffMap[*sub.RegularStaffID]; ok {
				sub.RegularStaff = staff
			}
		}
		if staff, ok := staffMap[sub.SubstituteStaffID]; ok {
			sub.SubstituteStaff = staff
		}
	}

	return substitutions, nil
}

// FindActiveWithRelations retrieves all active substitutions for a specific date with related data
func (r *GroupSubstitutionRepository) FindActiveWithRelations(ctx context.Context, date time.Time) ([]*education.GroupSubstitution, error) {
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()
	filter.DateBetween("start_date", "end_date", date)
	options.Filter = filter

	return r.ListWithRelations(ctx, options)
}

// FindActiveBySubstituteWithRelations retrieves active substitutions for a staff member and date with related data
func (r *GroupSubstitutionRepository) FindActiveBySubstituteWithRelations(ctx context.Context, substituteStaffID int64, date time.Time) ([]*education.GroupSubstitution, error) {
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()
	filter.Equal("substitute_staff_id", substituteStaffID)
	filter.DateBetween("start_date", "end_date", date)
	options.Filter = filter

	return r.ListWithRelations(ctx, options)
}

// FindActiveByGroupWithRelations retrieves active substitutions for a specific group and date with related data
func (r *GroupSubstitutionRepository) FindActiveByGroupWithRelations(ctx context.Context, groupID int64, date time.Time) ([]*education.GroupSubstitution, error) {
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()
	filter.Equal("group_id", groupID)
	filter.DateBetween("start_date", "end_date", date)
	options.Filter = filter

	return r.ListWithRelations(ctx, options)
}
