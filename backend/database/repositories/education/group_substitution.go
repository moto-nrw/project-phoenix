// backend/database/repositories/education/group_substitution.go
package education

import (
	"context"
	"maps"
	"slices"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Table name constant to avoid string literal duplication
const tableGroupSubstitution = "education.group_substitution"

// Aliased table expression for custom queries with tenant filtering
const tableExprGroupSubstitutionAsGS = `education.group_substitution AS "group_substitution"`

// Query constants (S1192 - avoid duplicate string literals)
const dateRangeContainsCondition = "start_date <= ? AND end_date >= ?"

// GroupSubstitutionRepository implements education.GroupSubstitutionRepository interface
type GroupSubstitutionRepository struct {
	*base.Repository[*education.GroupSubstitution]
	db *bun.DB
}

// NewGroupSubstitutionRepository creates a new GroupSubstitutionRepository
func NewGroupSubstitutionRepository(db *bun.DB) education.GroupSubstitutionRepository {
	repo := base.NewRepository[*education.GroupSubstitution](db, tableGroupSubstitution, "group_substitution")
	repo.TenantScoped = true
	return &GroupSubstitutionRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByGroup retrieves all substitutions for a specific group
func (r *GroupSubstitutionRepository) FindByGroup(ctx context.Context, groupID int64) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&substitutions).
		ModelTableExpr(tableExprGroupSubstitutionAsGS).
		Where(`"group_substitution".group_id = ?`, groupID)

	query = base.WithTenantFilter(ctx, query, "group_substitution")

	err := query.Scan(ctx)
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&substitutions).
		ModelTableExpr(tableExprGroupSubstitutionAsGS).
		Where(`"group_substitution".regular_staff_id = ?`, staffID)

	query = base.WithTenantFilter(ctx, query, "group_substitution")

	err := query.Scan(ctx)
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&substitutions).
		ModelTableExpr(tableExprGroupSubstitutionAsGS).
		Where(`"group_substitution".substitute_staff_id = ?`, staffID)

	query = base.WithTenantFilter(ctx, query, "group_substitution")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by substitute staff",
			Err: err,
		}
	}

	return substitutions, nil
}

// DeleteActiveOrFutureByStaffID removes substitutions involving the staff
// member (as regular or substitute) that have not ended before the given date.
// Past substitutions stay as history. Used by staff offboarding, where the
// staff row is only soft-deleted and the old ON DELETE CASCADE therefore no
// longer cleans up assignments.
func (r *GroupSubstitutionRepository) DeleteActiveOrFutureByStaffID(ctx context.Context, staffID int64, from timezone.Date) (int64, error) {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*education.GroupSubstitution)(nil)).
		ModelTableExpr(tableExprGroupSubstitutionAsGS).
		Where(`("group_substitution".regular_staff_id = ? OR "group_substitution".substitute_staff_id = ?)`, staffID, staffID).
		Where(`"group_substitution".end_date >= ?`, from)

	query = base.WithTenantFilter(ctx, query, "group_substitution")

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete active or future by staff id",
			Err: err,
		}
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

// FindActive retrieves all active substitutions for a specific date
func (r *GroupSubstitutionRepository) FindActive(ctx context.Context, date timezone.Date) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&substitutions).
		ModelTableExpr(tableExprGroupSubstitutionAsGS).
		Where(`"group_substitution".`+dateRangeContainsCondition, date, date)

	query = base.WithTenantFilter(ctx, query, "group_substitution")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active",
			Err: err,
		}
	}

	return substitutions, nil
}

// FindActiveBySubstitute retrieves all active substitutions for a staff member and date
func (r *GroupSubstitutionRepository) FindActiveBySubstitute(ctx context.Context, substituteStaffID int64, date timezone.Date) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&substitutions).
		ModelTableExpr(tableExprGroupSubstitutionAsGS).
		Where(`"group_substitution".substitute_staff_id = ?`, substituteStaffID).
		Where(`"group_substitution".`+dateRangeContainsCondition, date, date)

	query = base.WithTenantFilter(ctx, query, "group_substitution")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by substitute",
			Err: err,
		}
	}

	return substitutions, nil
}

// FindOverlapping finds all substitutions that overlap with the given date range for a staff member
func (r *GroupSubstitutionRepository) FindOverlapping(ctx context.Context, staffID int64, startDate timezone.Date, endDate timezone.Date) ([]*education.GroupSubstitution, error) {
	var substitutions []*education.GroupSubstitution
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&substitutions).
		ModelTableExpr(tableExprGroupSubstitutionAsGS).
		Where(`("group_substitution".regular_staff_id = ? OR "group_substitution".substitute_staff_id = ?)`, staffID, staffID).
		Where(`"group_substitution".start_date <= ? AND "group_substitution".end_date >= ?`, endDate, startDate)

	query = base.WithTenantFilter(ctx, query, "group_substitution")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find overlapping",
			Err: err,
		}
	}

	return substitutions, nil
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

// applyActiveFilter applies active date filter using today's Berlin calendar day
func applyActiveFilter(filter *modelBase.Filter, value interface{}) {
	if boolValue, ok := value.(bool); ok && boolValue {
		filter.DateBetween("start_date", "end_date", timezone.TodayDate())
	}
}

// applyDateFilter applies date filter for a specific date
func applyDateFilter(filter *modelBase.Filter, value interface{}) {
	if dateValue, ok := value.(timezone.Date); ok {
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
	rows, err := r.Repository.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = make([]*education.GroupSubstitution, 0)
	}
	return rows, nil
}

// FindByIDWithRelations retrieves a substitution by ID with all related data loaded
func (r *GroupSubstitutionRepository) FindByIDWithRelations(ctx context.Context, id int64) (*education.GroupSubstitution, error) {
	var substitution education.GroupSubstitution

	mainQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(&substitution).
		ModelTableExpr(tableExprGroupSubstitutionAsGS).
		Where(`"group_substitution".id = ?`, id)

	mainQuery = base.WithTenantFilter(ctx, mainQuery, "group_substitution")

	err := mainQuery.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by id with relations",
			Err: err,
		}
	}

	// Load group
	if substitution.GroupID > 0 {
		var group education.Group
		err = base.GetDB(ctx, r.db).NewSelect().
			Model(&group).
			ModelTableExpr(`education.groups AS "group"`).
			Where(`"group".id = ?`, substitution.GroupID).
			Scan(ctx)
		if err == nil {
			substitution.Group = &group
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

	groupIDSlice := slices.Collect(maps.Keys(groupIDs))

	var groups []*education.Group
	groupQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(`education.groups AS "group"`).
		Where(`"group".id IN (?)`, bun.List(groupIDSlice))

	groupQuery = base.WithTenantFilter(ctx, groupQuery, "group")

	err := groupQuery.Scan(ctx)
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

	staffIDSlice := slices.Collect(maps.Keys(staffIDs))

	// Load staff records. Include soft-deleted staff so historical
	// substitutions keep resolving the staff member's name after offboarding.
	var staffList []*users.Staff
	staffQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(&staffList).
		ModelTableExpr(`users.staff AS "staff"`).
		WhereAllWithDeleted().
		Where(`"staff".id IN (?)`, bun.List(staffIDSlice))

	staffQuery = base.WithTenantFilter(ctx, staffQuery, "staff")

	err := staffQuery.Scan(ctx)

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
	personQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(&persons).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".id IN (?)`, bun.List(personIDs))

	personQuery = base.WithTenantFilter(ctx, personQuery, "person")

	err := personQuery.Scan(ctx)

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

// FindActiveWithRelations retrieves all active substitutions for a specific date with related data
func (r *GroupSubstitutionRepository) FindActiveWithRelations(ctx context.Context, date timezone.Date) ([]*education.GroupSubstitution, error) {
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()
	filter.DateBetween("start_date", "end_date", date)
	options.Filter = filter

	return r.ListWithRelations(ctx, options)
}

// FindActiveBySubstituteWithRelations retrieves active substitutions for a staff member and date with related data
func (r *GroupSubstitutionRepository) FindActiveBySubstituteWithRelations(ctx context.Context, substituteStaffID int64, date timezone.Date) ([]*education.GroupSubstitution, error) {
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()
	filter.Equal("substitute_staff_id", substituteStaffID)
	filter.DateBetween("start_date", "end_date", date)
	options.Filter = filter

	return r.ListWithRelations(ctx, options)
}

// FindActiveByGroupWithRelations retrieves active substitutions for a specific group and date with related data
func (r *GroupSubstitutionRepository) FindActiveByGroupWithRelations(ctx context.Context, groupID int64, date timezone.Date) ([]*education.GroupSubstitution, error) {
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()
	filter.Equal("group_id", groupID)
	filter.DateBetween("start_date", "end_date", date)
	options.Filter = filter

	return r.ListWithRelations(ctx, options)
}

// ListActiveSubstitutionBlockers returns the staff member's current or
// upcoming substitutions (as substitute or regular) as caregiver-capability
// blocker rows. Custom raw-SQL method (backend-conventions Rule 2):
// role CASE projection into the users blocker read model.
func (r *GroupSubstitutionRepository) ListActiveSubstitutionBlockers(ctx context.Context, staffID, tenantID int64) ([]users.BlockerSubstitution, error) {
	var results []users.BlockerSubstitution
	err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT gs.id, COALESCE(g.name, 'Unbekannte Gruppe') AS group_name,
		       CASE WHEN gs.substitute_staff_id = ? THEN 'substitute' ELSE 'regular' END AS role,
		       gs.start_date::text AS start_date,
		       gs.end_date::text AS end_date
		FROM education.group_substitution AS gs
		LEFT JOIN education.groups AS g ON g.id = gs.group_id AND g.tenant_id = gs.tenant_id
		WHERE gs.tenant_id = ?
		  AND (gs.substitute_staff_id = ? OR gs.regular_staff_id = ?)
		  AND gs.end_date >= CURRENT_DATE
		ORDER BY gs.start_date DESC
	`, staffID, tenantID, staffID, staffID).Scan(ctx, &results)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list active substitution blockers",
			Err: err,
		}
	}
	return results, nil
}
