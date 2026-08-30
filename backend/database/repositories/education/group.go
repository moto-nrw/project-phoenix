// backend/database/repositories/education/group.go
package education

import (
	"context"

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

// ListStaffIDsByEducationGroupIDs returns the (staff, group) pairs supervising
// the given groups on the given day.
//
// The bulk mirror of usercontext.GetMyGroups read from the group side,
// deliberately built from the same two sources with the same predicates:
// teacher assignments plus substitutions active on the day, inner joins
// throughout, soft-deleted staff and teachers excluded.
func (r *GroupRepository) ListStaffIDsByEducationGroupIDs(ctx context.Context, groupIDs []int64, on timezone.Date) ([]education.StaffGroupID, error) {
	if len(groupIDs) == 0 {
		return []education.StaffGroupID{}, nil
	}

	pairs := make([]education.StaffGroupID, 0, len(groupIDs))
	seen := make(map[education.StaffGroupID]struct{}, len(groupIDs))

	appendRows := func(rows []education.StaffGroupID) {
		for _, row := range rows {
			if _, dup := seen[row]; dup {
				continue
			}
			seen[row] = struct{}{}
			pairs = append(pairs, row)
		}
	}

	var assigned []education.StaffGroupID
	assignedQuery := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`education.groups AS "group"`).
		ColumnExpr(`"staff".id AS staff_id, "group".id AS group_id`).
		Join(`JOIN education.group_teacher AS "gt" ON "gt".group_id = "group".id`).
		Join(`JOIN users.teachers AS "teacher" ON "teacher".id = "gt".teacher_id AND "teacher".deleted_at IS NULL`).
		Join(`JOIN users.staff AS "staff" ON "staff".id = "teacher".staff_id AND "staff".deleted_at IS NULL`).
		Where(`"group".id IN (?)`, bun.List(groupIDs))

	assignedQuery = base.WithTenantFilter(ctx, assignedQuery, "group")

	if err := assignedQuery.Scan(ctx, &assigned); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list staff IDs by education group IDs (assigned)",
			Err: err,
		}
	}
	appendRows(assigned)

	var substituted []education.StaffGroupID
	substitutedQuery := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`education.group_substitution AS "sub"`).
		ColumnExpr(`"staff".id AS staff_id, "sub".group_id AS group_id`).
		Join(`JOIN users.staff AS "staff" ON "staff".id = "sub".substitute_staff_id AND "staff".deleted_at IS NULL`).
		Where(`"sub".group_id IN (?)`, bun.List(groupIDs)).
		Where(`"sub".start_date <= ?`, on).
		Where(`"sub".end_date >= ?`, on)

	substitutedQuery = base.WithTenantFilter(ctx, substitutedQuery, "sub")

	if err := substitutedQuery.Scan(ctx, &substituted); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list staff IDs by education group IDs (substitutions)",
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

// FindByIDsWithRooms retrieves groups (keyed by ID) with their room relation
// preloaded via one LEFT JOIN — the bulk sibling of FindWithRoom, added so the
// OGS live projection resolves every supervised group's room name in a single
// query instead of one per group (#2094 review).
func (r *GroupRepository) FindByIDsWithRooms(ctx context.Context, ids []int64) (map[int64]*education.Group, error) {
	result := make(map[int64]*education.Group, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	type row struct {
		*education.Group `bun:",extend"`
		Room             *facilities.Room `bun:"rel:belongs-to,join:room_id=id"`
	}

	var rows []row
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`education.groups AS "group"`).
		ColumnExpr(`"group".*`).
		ColumnExpr(`"room".id AS "room__id", "room".created_at AS "room__created_at", "room".updated_at AS "room__updated_at"`).
		ColumnExpr(`"room".name AS "room__name", "room".building AS "room__building", "room".floor AS "room__floor"`).
		ColumnExpr(`"room".capacity AS "room__capacity", "room".category AS "room__category", "room".color AS "room__color"`).
		Join(`LEFT JOIN facilities.rooms AS "room" ON "room".id = "group".room_id`).
		Where(`"group".id IN (?)`, bun.List(ids))

	query = base.WithTenantFilter(ctx, query, "group")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by IDs with rooms",
			Err: err,
		}
	}

	for _, r := range rows {
		group := r.Group
		if r.Room != nil && r.Room.ID != 0 {
			group.Room = r.Room
		}
		result[group.ID] = group
	}
	return result, nil
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

type groupWithRoom struct {
	Group *education.Group `bun:"group"`
	Room  *facilities.Room `bun:"room"`
}

// ListWithRooms lists groups and their optional room in one joined snapshot.
func (r *GroupRepository) ListWithRooms(ctx context.Context, params *education.GroupListQuery) ([]*education.Group, error) {
	results := make([]groupWithRoom, 0)
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
	filter := params.Filter().WithTableAlias("group")
	query = filter.ApplyToQuery(query)
	if params != nil {
		if params.SortByName {
			if params.Descending {
				query = query.OrderExpr(`"group".name DESC`)
			} else {
				query = query.OrderExpr(`"group".name ASC`)
			}
		}
		if params.Limit > 0 {
			query = query.Limit(params.Limit).Offset(params.Offset)
		}
	}
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list with options", Err: err}
	}

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
