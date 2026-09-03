// backend/database/repositories/education/group_substitution.go
package education

import (
	"context"
	"errors"
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
	// substitutionStaff attaches RegularStaff and SubstituteStaff through
	// School Membership, which owns users.staff (#2667). Soft-deleted staff
	// are expected to resolve so historical substitutions keep their names.
	substitutionStaff func(ctx context.Context, substitutions []*education.GroupSubstitution) error
}

// SetSubstitutionStaffResolver installs the staff lookup used by every
// relation-loading read.
func (r *GroupSubstitutionRepository) SetSubstitutionStaffResolver(resolve func(ctx context.Context, substitutions []*education.GroupSubstitution) error) {
	r.substitutionStaff = resolve
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
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
		date := timezone.TodayDate()
		filter.LessThanOrEqual("start_date", date).GreaterThanOrEqual("end_date", date)
	}
}

// applyDateFilter applies date filter for a specific date
func applyDateFilter(filter *modelBase.Filter, value interface{}) {
	if dateValue, ok := value.(timezone.Date); ok {
		filter.LessThanOrEqual("start_date", dateValue).GreaterThanOrEqual("end_date", dateValue)
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

// ListWithRelations retrieves substitutions with all related data loaded
func (r *GroupSubstitutionRepository) ListWithRelations(ctx context.Context, options *modelBase.QueryOptions) ([]*education.GroupSubstitution, error) {
	substitutions, err := r.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}

	// Collect unique IDs
	groupIDs := collectSubstitutionGroupIDs(substitutions)

	// Load all related data
	groupMap, err := r.loadGroupsByIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}

	assignGroupsToSubstitutions(substitutions, groupMap)

	// The staff behind the substitution belongs to School Membership; the
	// injected resolver attaches it (#2667). Fail closed: without it every
	// substitution would render nameless, which is wrong data rather than an
	// obvious outage.
	if r.substitutionStaff == nil {
		return nil, errors.New("group substitution repository resolves staff through School Membership")
	}
	if err := r.substitutionStaff(ctx, substitutions); err != nil {
		return nil, err
	}

	return substitutions, nil
}

// collectSubstitutionGroupIDs extracts the unique group IDs from substitutions
func collectSubstitutionGroupIDs(substitutions []*education.GroupSubstitution) map[int64]bool {
	groupIDs := make(map[int64]bool)

	for _, sub := range substitutions {
		if sub.GroupID > 0 {
			groupIDs[sub.GroupID] = true
		}
	}

	return groupIDs
}

// loadGroupsByIDs loads groups by their IDs and returns a map
func (r *GroupSubstitutionRepository) loadGroupsByIDs(ctx context.Context, groupIDs map[int64]bool) (map[int64]*education.Group, error) {
	groupMap := make(map[int64]*education.Group)
	if len(groupIDs) == 0 {
		return groupMap, nil
	}

	groupIDSlice := slices.Collect(maps.Keys(groupIDs))

	var groups []*education.Group
	groupQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(`education.groups AS "group"`).
		Where(`"group".id IN (?)`, bun.List(groupIDSlice))

	groupQuery = base.WithTenantFilter(ctx, groupQuery, "group")

	if err := groupQuery.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "load substitution groups", Err: base.TranslateNotFound(err)}
	}
	for _, group := range groups {
		groupMap[group.ID] = group
	}

	return groupMap, nil
}

// assignGroupsToSubstitutions assigns the loaded groups to substitution records
func assignGroupsToSubstitutions(substitutions []*education.GroupSubstitution, groupMap map[int64]*education.Group) {
	for _, sub := range substitutions {
		if group, ok := groupMap[sub.GroupID]; ok {
			sub.Group = group
		}
	}
}

// FindActiveBySubstituteWithRelations retrieves active substitutions for a staff member and date with related data
func (r *GroupSubstitutionRepository) FindActiveBySubstituteWithRelations(ctx context.Context, substituteStaffID int64, date timezone.Date) ([]*education.GroupSubstitution, error) {
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()
	filter.Equal("substitute_staff_id", substituteStaffID)
	filter.LessThanOrEqual("start_date", date).GreaterThanOrEqual("end_date", date)
	options.Filter = filter

	return r.ListWithRelations(ctx, options)
}

// ListActiveSubstitutionBlockers returns the staff member's current or
// upcoming typed group handovers as caregiver-capability blocker rows.
// Custom raw-SQL method (backend-conventions Rule 2).
func (r *GroupSubstitutionRepository) ListActiveSubstitutionBlockers(ctx context.Context, staffID, tenantID int64) ([]users.BlockerSubstitution, error) {
	var results []users.BlockerSubstitution
	err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT gs.id, COALESCE(g.name, 'Unbekannte Gruppe') AS group_name,
		       gs.start_date::text AS start_date,
		       gs.end_date::text AS end_date
		FROM education.group_substitution AS gs
		LEFT JOIN education.groups AS g ON g.id = gs.group_id AND g.tenant_id = gs.tenant_id
		WHERE gs.tenant_id = ?
		  AND gs.target_type = 'group_handover'
		  AND gs.substitute_staff_id = ?
		  AND gs.end_date >= CURRENT_DATE
		ORDER BY gs.start_date DESC
	`, tenantID, staffID).Scan(ctx, &results)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list active substitution blockers",
			Err: base.TranslateNotFound(err),
		}
	}
	return results, nil
}
