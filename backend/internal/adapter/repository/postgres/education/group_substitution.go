// backend/database/repositories/education/group_substitution.go
package education

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
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

// applySubstitutionFilter applies a single filter based on field name
func applySubstitutionFilter(filter *modelBase.Filter, field string, value interface{}) {
	switch field {
	case "active":
		applyActiveFilter(filter, value)
	case "date":
		applyDateFilter(filter, value)
	case "reason_like":
		applyReasonLikeFilter(filter, value)
	default:
		filter.Equal(field, value)
	}
}

// applyActiveFilter applies active date filter using current time
func applyActiveFilter(filter *modelBase.Filter, value interface{}) {
	if boolValue, ok := value.(bool); ok && boolValue {
		filter.DateBetween("start_date", "end_date", time.Now())
	}
}

// applyDateFilter applies date filter for a specific date
func applyDateFilter(filter *modelBase.Filter, value interface{}) {
	if dateValue, ok := value.(time.Time); ok {
		filter.DateBetween("start_date", "end_date", dateValue)
	}
}

// applyReasonLikeFilter applies LIKE filter for reason field
func applyReasonLikeFilter(filter *modelBase.Filter, value interface{}) {
	if strValue, ok := value.(string); ok {
		filter.ILike("reason", "%"+strValue+"%")
	}
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
	substitutions, err := r.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}

	// Collect unique IDs
	groupIDs, staffIDs := collectSubstitutionRelatedIDs(substitutions)

	// Load all related data
	groupMap := r.loadGroupsByIDs(ctx, groupIDs)
	staffMap := r.loadStaffWithPersonsByIDs(ctx, staffIDs)

	// Assign loaded data to substitutions
	assignRelationsToSubstitutions(substitutions, groupMap, staffMap)

	return substitutions, nil
}

// collectSubstitutionRelatedIDs extracts unique group and staff IDs from substitutions
func collectSubstitutionRelatedIDs(substitutions []*education.GroupSubstitution) (groupIDs, staffIDs map[int64]bool) {
	groupIDs = make(map[int64]bool)
	staffIDs = make(map[int64]bool)

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

	return groupIDs, staffIDs
}

// loadGroupsByIDs loads groups by their IDs and returns a map
func (r *GroupSubstitutionRepository) loadGroupsByIDs(ctx context.Context, groupIDs map[int64]bool) map[int64]*education.Group {
	groupMap := make(map[int64]*education.Group)
	if len(groupIDs) == 0 {
		return groupMap
	}

	groupIDSlice := mapKeysToSlice(groupIDs)

	var groups []*education.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`education.groups AS "group"`).
		Where(`"group".id IN (?)`, bun.In(groupIDSlice)).
		Scan(ctx)

	if err == nil {
		for _, group := range groups {
			groupMap[group.ID] = group
		}
	}

	return groupMap
}

// loadStaffWithPersonsByIDs loads staff with their persons by IDs
func (r *GroupSubstitutionRepository) loadStaffWithPersonsByIDs(ctx context.Context, staffIDs map[int64]bool) map[int64]*users.Staff {
	staffMap := make(map[int64]*users.Staff)
	if len(staffIDs) == 0 {
		return staffMap
	}

	staffIDSlice := mapKeysToSlice(staffIDs)

	// Load staff records
	var staffList []*users.Staff
	err := r.db.NewSelect().
		Model(&staffList).
		ModelTableExpr(`users.staff AS "staff"`).
		Where(`"staff".id IN (?)`, bun.In(staffIDSlice)).
		Scan(ctx)

	if err != nil || len(staffList) == 0 {
		return staffMap
	}

	// Build staff map and collect person IDs
	personIDs := make([]int64, 0, len(staffList))
	for _, staff := range staffList {
		staffMap[staff.ID] = staff
		if staff.PersonID > 0 {
			personIDs = append(personIDs, staff.PersonID)
		}
	}

	// Load and link persons
	r.linkPersonsToStaff(ctx, staffList, personIDs)

	return staffMap
}

// linkPersonsToStaff loads persons and links them to staff records
func (r *GroupSubstitutionRepository) linkPersonsToStaff(ctx context.Context, staffList []*users.Staff, personIDs []int64) {
	if len(personIDs) == 0 {
		return
	}

	var persons []*users.Person
	err := r.db.NewSelect().
		Model(&persons).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".id IN (?)`, bun.In(personIDs)).
		Scan(ctx)

	if err != nil {
		return
	}

	personMap := make(map[int64]*users.Person)
	for _, person := range persons {
		personMap[person.ID] = person
	}

	for _, staff := range staffList {
		if person, ok := personMap[staff.PersonID]; ok {
			staff.Person = person
		}
	}
}

// assignRelationsToSubstitutions assigns loaded relations to substitution records
func assignRelationsToSubstitutions(substitutions []*education.GroupSubstitution, groupMap map[int64]*education.Group, staffMap map[int64]*users.Staff) {
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
}

// mapKeysToSlice converts map keys to a slice
func mapKeysToSlice(m map[int64]bool) []int64 {
	slice := make([]int64, 0, len(m))
	for id := range m {
		slice = append(slice, id)
	}
	return slice
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
