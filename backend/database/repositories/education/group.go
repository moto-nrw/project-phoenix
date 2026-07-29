// backend/database/repositories/education/group.go
package education

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// GroupRepository implements education.GroupRepository interface
type GroupRepository struct {
	*base.Repository[*education.Group]
	db *bun.DB
}

// NewGroupRepository creates a new GroupRepository
func NewGroupRepository(db *bun.DB) education.GroupRepository {
	repo := base.NewRepository[*education.Group](db, "education.groups", "Group")
	repo.TenantScoped = true
	return &GroupRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByName retrieves a group by its name
func (r *GroupRepository) FindByName(ctx context.Context, name string) (*education.Group, error) {
	group := new(education.Group)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(group).
		ModelTableExpr(`education.groups AS "group"`).
		Where("LOWER(name) = LOWER(?)", name)

	query = base.WithTenantFilter(ctx, query, "group")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by name",
			Err: err,
		}
	}

	return group, nil
}

// FindByIDs retrieves multiple groups by their IDs in a single query
func (r *GroupRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*education.Group, error) {
	if len(ids) == 0 {
		return make(map[int64]*education.Group), nil
	}

	var groups []*education.Group
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(`education.groups AS "group"`).
		Where(`"group".id IN (?)`, bun.List(ids))

	query = base.WithTenantFilter(ctx, query, "group")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by IDs",
			Err: err,
		}
	}

	// Convert to map for O(1) lookups
	result := make(map[int64]*education.Group, len(groups))
	for _, group := range groups {
		result[group.ID] = group
	}

	return result, nil
}

// FindByTeacher retrieves groups by their teacher ID (via group_teacher table)
func (r *GroupRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*education.Group, error) {
	var groups []*education.Group
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(`education.groups AS "group"`).
		Join("JOIN education.group_teacher gt ON gt.group_id = \"group\".id").
		Where("gt.teacher_id = ?", teacherID)

	query = base.WithTenantFilter(ctx, query, "group")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by teacher",
			Err: err,
		}
	}

	return groups, nil
}

// ListSupervisedGroupIDsByStaff returns the (staff, education group) pairs the
// given staff members supervise on the given day.
//
// It mirrors usercontext.GetMyGroups exactly, in bulk and without needing JWT
// claims in the context. GetMyGroups resolves person -> staff -> teacher ->
// groups and unions that with the staff member's active substitutions; the two
// branches below are the same two sources, so an equivalence test can pin them
// against each other.
//
// Two queries rather than one UNION: the tenant predicate is applied per branch
// through the standard helper, which a hand-written UNION would force us to
// inline. The cost that matters is that neither branch scales with the number
// of staff members, and both are IN-list lookups.
//
// Every join is an INNER join on purpose. A LEFT join here would emit rows with
// a NULL group for a staff member who supervises nothing, and a caller reading
// "no rows means no restriction" would turn this filter into full access.
// Soft-deleted staff and teachers are excluded for the same reason GetMyGroups
// returns nothing for them: their identity lookup fails, so they supervise
// nothing.
func (r *GroupRepository) ListSupervisedGroupIDsByStaff(ctx context.Context, staffIDs []int64, on timezone.Date) ([]education.StaffGroupID, error) {
	if len(staffIDs) == 0 {
		return []education.StaffGroupID{}, nil
	}

	pairs := make([]education.StaffGroupID, 0, len(staffIDs))
	seen := make(map[education.StaffGroupID]struct{}, len(staffIDs))

	appendRows := func(rows []education.StaffGroupID) {
		for _, row := range rows {
			if _, dup := seen[row]; dup {
				continue
			}
			seen[row] = struct{}{}
			pairs = append(pairs, row)
		}
	}

	// Branch 1: groups the staff member is assigned to as a teacher. Mirrors
	// GetCurrentStaff -> teacherRepo.FindByStaffID -> educationGroupRepo.FindByTeacher.
	var assigned []education.StaffGroupID
	assignedQuery := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.staff AS "staff"`).
		ColumnExpr(`"staff".id AS staff_id, "group".id AS group_id`).
		Join(`JOIN users.teachers AS "teacher" ON "teacher".staff_id = "staff".id AND "teacher".deleted_at IS NULL`).
		Join(`JOIN education.group_teacher AS "gt" ON "gt".teacher_id = "teacher".id`).
		Join(`JOIN education.groups AS "group" ON "group".id = "gt".group_id`).
		Where(`"staff".deleted_at IS NULL`).
		Where(`"staff".id IN (?)`, bun.List(staffIDs))

	assignedQuery = base.WithTenantFilter(ctx, assignedQuery, "staff")

	if err := assignedQuery.Scan(ctx, &assigned); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list supervised group IDs by staff (assigned)",
			Err: err,
		}
	}
	appendRows(assigned)

	// Branch 2: groups covered through a substitution that is active on `on`.
	// The date predicate is inclusive on both ends, matching Filter.DateBetween
	// as used by FindActiveBySubstituteWithRelations. The join back to
	// users.staff reproduces GetMyGroups' precondition that the substitute has
	// a live staff row at all.
	var substituted []education.StaffGroupID
	substitutedQuery := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`education.group_substitution AS "sub"`).
		ColumnExpr(`"sub".substitute_staff_id AS staff_id, "group".id AS group_id`).
		Join(`JOIN users.staff AS "staff" ON "staff".id = "sub".substitute_staff_id AND "staff".deleted_at IS NULL`).
		Join(`JOIN education.groups AS "group" ON "group".id = "sub".group_id`).
		Where(`"sub".substitute_staff_id IN (?)`, bun.List(staffIDs)).
		Where(`"sub".start_date <= ?`, on).
		Where(`"sub".end_date >= ?`, on)

	substitutedQuery = base.WithTenantFilter(ctx, substitutedQuery, "sub")

	if err := substitutedQuery.Scan(ctx, &substituted); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list supervised group IDs by staff (substitutions)",
			Err: err,
		}
	}
	appendRows(substituted)

	return pairs, nil
}

// FindWithRoom retrieves a group with its associated room
func (r *GroupRepository) FindWithRoom(ctx context.Context, groupID int64) (*education.Group, error) {
	group := new(education.Group)

	// Perform manual join to avoid schema issues with Relation()
	type Result struct {
		*education.Group `bun:",extend"`
		Room             *facilities.Room `bun:"rel:belongs-to,join:room_id=id"`
	}

	result := new(Result)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(result).
		ModelTableExpr(`education.groups AS "group"`).
		ColumnExpr(`"group".*`).
		ColumnExpr(`"room".id AS "room__id", "room".created_at AS "room__created_at", "room".updated_at AS "room__updated_at"`).
		ColumnExpr(`"room".name AS "room__name", "room".building AS "room__building", "room".floor AS "room__floor"`).
		ColumnExpr(`"room".capacity AS "room__capacity", "room".category AS "room__category", "room".color AS "room__color"`).
		Join(`LEFT JOIN facilities.rooms AS "room" ON "room".id = "group".room_id`).
		Where(`"group".id = ?`, groupID)

	query = base.WithTenantFilter(ctx, query, "group")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with room",
			Err: err,
		}
	}

	// Map result to group
	group = result.Group
	if result.Room != nil && result.Room.ID != 0 {
		group.Room = result.Room
	}

	return group, nil
}

// List retrieves groups matching the provided query options
func (r *GroupRepository) List(ctx context.Context, filters map[string]interface{}) ([]*education.Group, error) {
	options := modelBase.NewQueryOptions()
	options.Filter = buildGroupFilter(filters)
	return r.ListWithOptions(ctx, options)
}

// buildGroupFilter converts legacy filter map to QueryOptions filter
func buildGroupFilter(filters map[string]interface{}) *modelBase.Filter {
	filter := modelBase.NewFilter()
	for field, value := range filters {
		if value == nil {
			continue
		}
		applyGroupFilterField(filter, field, value)
	}
	return filter
}

// applyGroupFilterField applies a single filter field
func applyGroupFilterField(filter *modelBase.Filter, field string, value interface{}) {
	switch field {
	case "name_like":
		if strValue, ok := value.(string); ok {
			filter.ILike("name", "%"+strValue+"%")
		}
	case "has_room":
		if boolValue, ok := value.(bool); ok {
			if boolValue {
				filter.IsNotNull("room_id")
			} else {
				filter.IsNull("room_id")
			}
		}
	default:
		filter.Equal(field, value)
	}
}

// groupWithRoom is used to scan LEFT JOIN results for group + room data
type groupWithRoom struct {
	Group *education.Group `bun:"group"`
	Room  *facilities.Room `bun:"room"`
}

// ListWithOptions provides a type-safe way to list groups with query options
func (r *GroupRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*education.Group, error) {
	var results []groupWithRoom
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&results).
		ModelTableExpr(`education.groups AS "group"`).
		ColumnExpr(`"group".id AS "group__id", "group".created_at AS "group__created_at", "group".updated_at AS "group__updated_at"`).
		ColumnExpr(`"group".tenant_id AS "group__tenant_id", "group".name AS "group__name", "group".room_id AS "group__room_id"`).
		ColumnExpr(`"room".id AS "room__id", "room".created_at AS "room__created_at", "room".updated_at AS "room__updated_at"`).
		ColumnExpr(`"room".name AS "room__name", "room".building AS "room__building", "room".floor AS "room__floor"`).
		ColumnExpr(`"room".capacity AS "room__capacity", "room".category AS "room__category", "room".color AS "room__color"`).
		Join(`LEFT JOIN facilities.rooms AS "room" ON "room".id = "group".room_id`)

	query = base.WithTenantFilter(ctx, query, "group")

	// Apply query options (ensure table alias for JOINed queries to avoid ambiguous columns)
	if options != nil {
		if options.Filter != nil {
			options.Filter.WithTableAlias("group")
		}
		if options.Sorting != nil {
			for i, f := range options.Sorting.Fields {
				if !strings.Contains(f.Field, ".") {
					options.Sorting.Fields[i].Field = `group.` + f.Field
				}
			}
		}
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list with options",
			Err: err,
		}
	}

	// Map results back to []*education.Group with room attached
	groups := make([]*education.Group, len(results))
	for i, result := range results {
		groups[i] = result.Group
		if result.Room != nil && result.Room.ID != 0 {
			groups[i].Room = result.Room
		}
	}

	return groups, nil
}

// Exists reports whether a group with the given ID exists in the current
// tenant (issue #584: moved verbatim from api/timetable template validation).
// Custom method (Rule 2): the generic shape has no EXISTS projection — going
// through List/Count would fetch or aggregate rows just to learn a boolean.
func (r *GroupRepository) Exists(ctx context.Context, id int64) (bool, error) {
	tenantID := tenant.FromContext(ctx)
	return base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`education.groups AS "group"`).
		Where(`"group".tenant_id = ?`, tenantID).
		Where(`"group".id = ?`, id).
		Exists(ctx)
}
