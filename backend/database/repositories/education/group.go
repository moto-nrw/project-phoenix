// backend/database/repositories/education/group.go
package education

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// GroupRepository implements education.GroupRepository interface
type GroupRepository struct {
	*base.Repository[*education.Group]
	db *bun.DB
	// rooms resolves Group.Room through the Facilities owner (#2665).
	rooms RoomDirectory
	// supervisionStaff resolves the raw supervision references of a group to
	// the staff members behind them, through School Membership (#2667).
	supervisionStaff func(ctx context.Context, pairs GroupMembershipPairs) ([]education.StaffGroupID, error)
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
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
			Err: base.TranslateNotFound(err),
		}
	}

	return groups, nil
}

// TeacherGroupID pairs a teacher profile with one education group they are
// assigned to. The teacher is resolved to a staff member by the composition
// layer.
type TeacherGroupID struct {
	TeacherID int64 `bun:"teacher_id"`
	GroupID   int64 `bun:"group_id"`
}

// GroupMembershipPairs are the raw supervision references of a set of
// education groups: teacher assignments and substitutions, both unresolved.
// The composition layer turns them into (staff, group) pairs through School
// Membership, dropping references to offboarded teachers and staff (#2667).
type GroupMembershipPairs struct {
	// Assigned pairs a group with the teacher assigned to it.
	Assigned []TeacherGroupID
	// Substituted pairs a group with the staff member substituting in it
	// on the requested day.
	Substituted []education.StaffGroupID
}

// SetSupervisionStaffResolver installs the lookup that turns the raw
// supervision references into (staff, group) pairs. School Membership owns
// users.teachers and users.staff, so the composition root injects it instead
// of this repository joining those tables (#2667).
func (r *GroupRepository) SetSupervisionStaffResolver(resolve func(ctx context.Context, pairs GroupMembershipPairs) ([]education.StaffGroupID, error)) {
	r.supervisionStaff = resolve
}

// listGroupMembershipPairs returns the unresolved supervision references of
// the given groups on the given day.
//
// The bulk mirror of usercontext.GetMyGroups read from the group side, built
// from the same two sources with the same predicates: teacher assignments
// plus substitutions active on the day.
func (r *GroupRepository) listGroupMembershipPairs(ctx context.Context, groupIDs []int64, on timezone.Date) (GroupMembershipPairs, error) {
	var pairs GroupMembershipPairs
	if len(groupIDs) == 0 {
		return pairs, nil
	}

	assignedQuery := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`education.groups AS "group"`).
		ColumnExpr(`"gt".teacher_id AS teacher_id, "group".id AS group_id`).
		Join(`JOIN education.group_teacher AS "gt" ON "gt".group_id = "group".id`).
		Where(`"group".id IN (?)`, bun.List(groupIDs))

	assignedQuery = base.WithTenantFilter(ctx, assignedQuery, "group")

	if err := assignedQuery.Scan(ctx, &pairs.Assigned); err != nil {
		return GroupMembershipPairs{}, &modelBase.DatabaseError{
			Op:  "list staff IDs by education group IDs (assigned)",
			Err: base.TranslateNotFound(err),
		}
	}

	substitutedQuery := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`education.group_substitution AS "sub"`).
		ColumnExpr(`"sub".substitute_staff_id AS staff_id, "sub".group_id AS group_id`).
		Where(`"sub".group_id IN (?)`, bun.List(groupIDs)).
		Where(`"sub".start_date <= ?`, on).
		Where(`"sub".end_date >= ?`, on)

	substitutedQuery = base.WithTenantFilter(ctx, substitutedQuery, "sub")

	if err := substitutedQuery.Scan(ctx, &pairs.Substituted); err != nil {
		return GroupMembershipPairs{}, &modelBase.DatabaseError{
			Op:  "list staff IDs by education group IDs (substitutions)",
			Err: base.TranslateNotFound(err),
		}
	}

	return pairs, nil
}

// ListStaffIDsByEducationGroupIDs returns the (staff, group) pairs
// supervising the given groups on the given day: teacher assignments plus
// substitutions active on that day, and nobody else. Resolving a teacher to
// their staff member, and dropping offboarded teachers and staff, is done by
// the injected School Membership lookup.
func (r *GroupRepository) ListStaffIDsByEducationGroupIDs(ctx context.Context, groupIDs []int64, on timezone.Date) ([]education.StaffGroupID, error) {
	if len(groupIDs) == 0 {
		return []education.StaffGroupID{}, nil
	}
	if r.supervisionStaff == nil {
		return nil, errors.New("group repository resolves supervising staff through School Membership")
	}
	pairs, err := r.listGroupMembershipPairs(ctx, groupIDs, on)
	if err != nil {
		return nil, err
	}
	return r.supervisionStaff(ctx, pairs)
}

// BindRoomDirectory installs the Facilities directory the room-enriched
// reads resolve Group.Room through (#2665).
func (r *GroupRepository) BindRoomDirectory(rooms RoomDirectory) {
	r.rooms = rooms
}

// FindWithRoom retrieves a group with its associated room
func (r *GroupRepository) FindWithRoom(ctx context.Context, groupID int64) (*education.Group, error) {
	group := new(education.Group)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(group).
		ModelTableExpr(`education.groups AS "group"`).
		Where(`"group".id = ?`, groupID)

	query = base.WithTenantFilter(ctx, query, "group")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with room",
			Err: base.TranslateNotFound(err),
		}
	}
	if err := attachRooms(ctx, r.rooms, []*education.Group{group}); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find with room", Err: err}
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

	var groups []*education.Group
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(`education.groups AS "group"`).
		Where(`"group".id IN (?)`, bun.List(ids))

	query = base.WithTenantFilter(ctx, query, "group")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by IDs with rooms",
			Err: base.TranslateNotFound(err),
		}
	}
	if err := attachRooms(ctx, r.rooms, groups); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by IDs with rooms", Err: err}
	}

	for _, group := range groups {
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

// ListWithRooms lists groups and their optional room in one snapshot: the
// groups from this owner, the rooms from Facilities (#2665).
func (r *GroupRepository) ListWithRooms(ctx context.Context, params *education.GroupListQuery) ([]*education.Group, error) {
	groups := make([]*education.Group, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(`education.groups AS "group"`)
	query = base.WithTenantFilter(ctx, query, "group")
	filter := params.Filter().WithTableAlias("group")
	query = base.ApplyFilter(query, filter)
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
		return nil, &modelBase.DatabaseError{Op: "list with options", Err: base.TranslateNotFound(err)}
	}
	if err := attachRooms(ctx, r.rooms, groups); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list with options", Err: err}
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
