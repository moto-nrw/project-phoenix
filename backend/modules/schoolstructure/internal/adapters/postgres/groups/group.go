package groups

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// groupTableExpr is the table and alias every group read and write uses.
const groupTableExpr = `education.groups AS "group"`

// groupRow is the persistence shape of education.groups.
type groupRow struct {
	bun.BaseModel `bun:"table:education.groups,alias:group"`
	ID            int64     `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	Name          string    `bun:"name,notnull"`
	RoomID        *int64    `bun:"room_id"`
}

func newGroupRow(group *domain.Group) *groupRow {
	return &groupRow{
		ID: group.ID, CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
		TenantID: group.TenantID, Name: group.Name, RoomID: group.RoomID,
	}
}

// copyTo writes the stored columns back onto the group, keeping its room.
func (row *groupRow) copyTo(group *domain.Group) {
	group.ID, group.CreatedAt, group.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
	group.TenantID, group.Name, group.RoomID = row.TenantID, row.Name, row.RoomID
}

func (row *groupRow) group() *domain.Group {
	group := &domain.Group{}
	row.copyTo(group)
	return group
}

func groupsOf(rows []*groupRow) []*domain.Group {
	groups := make([]*domain.Group, 0, len(rows))
	for _, row := range rows {
		groups = append(groups, row.group())
	}
	return groups
}

// errNilGroup keeps the generic repository's message for a missing group.
var errNilGroup = fmt.Errorf("%s cannot be nil or zero value", "Group")

// GroupRepository is the School Structure store of education.groups.
type GroupRepository struct {
	runtime Runtime
	// rooms resolves Group.Room through the Facilities owner (#2665).
	rooms func() RoomDirectory
	// supervisionStaff resolves the raw supervision references of a group to
	// the staff members behind them, through School Membership (#2667).
	supervisionStaff func(ctx context.Context, pairs GroupMembershipPairs) ([]domain.StaffGroupID, error)
	// assignments resolves education.group_teacher through composition. This
	// Postgres adapter stays independent of the sibling School Membership owner.
	assignments func(context.Context, []int64, []int64) ([]TeacherGroupID, error)
}

// NewGroupRepository creates a new GroupRepository on the tenant runtime.
func NewGroupRepository(runtime Runtime, rooms func() RoomDirectory, assignments func(context.Context, []int64, []int64) ([]TeacherGroupID, error), supervisionStaff func(context.Context, GroupMembershipPairs) ([]domain.StaffGroupID, error)) *GroupRepository {
	return &GroupRepository{runtime: requireRuntime(runtime), rooms: rooms, assignments: assignments, supervisionStaff: supervisionStaff}
}

// Create inserts a group. A group without a school takes the caller's.
func (r *GroupRepository) Create(ctx context.Context, group *domain.Group) error {
	if group == nil {
		return errNilGroup
	}
	if err := group.Validate(); err != nil {
		return err
	}
	if group.TenantID == 0 {
		group.TenantID = r.runtime.TenantID(ctx)
	}
	row := newGroupRow(group)
	if _, err := r.runtime.DB(ctx).NewInsert().
		Model(row).
		ModelTableExpr(`education.groups`).
		Exec(ctx); err != nil {
		return &domain.StoreError{Op: "create", Err: err}
	}
	row.copyTo(group)
	return nil
}

// FindByID retrieves a group by its ID.
func (r *GroupRepository) FindByID(ctx context.Context, id any) (*domain.Group, error) {
	return r.findByID(ctx, id, "find by id", false)
}

// FindByIDForUpdate is FindByID with a row lock held until the surrounding
// transaction finishes.
func (r *GroupRepository) FindByIDForUpdate(ctx context.Context, id any) (*domain.Group, error) {
	return r.findByID(ctx, id, "find by id for update", true)
}

func (r *GroupRepository) findByID(ctx context.Context, id any, op string, lock bool) (*domain.Group, error) {
	row := new(groupRow)
	query := r.runtime.DB(ctx).NewSelect().
		Model(row).
		ModelTableExpr(groupTableExpr).
		Where(`"group".id = ?`, id)
	if lock {
		query = query.For("UPDATE")
	}
	query = withTenantFilter(ctx, r.runtime, query, "group")
	if err := query.Scan(ctx); err != nil {
		return nil, &domain.StoreError{Op: op, Err: translateNotFound(err)}
	}
	return row.group(), nil
}

// Update writes a group's columns; exactly one row of the caller's school
// must change.
func (r *GroupRepository) Update(ctx context.Context, group *domain.Group) error {
	if group == nil {
		return errNilGroup
	}
	if err := group.Validate(); err != nil {
		return err
	}
	row := newGroupRow(group)
	query := r.runtime.DB(ctx).NewUpdate().
		Model(row).
		ModelTableExpr(groupTableExpr).
		WherePK()
	query = withTenantFilter(ctx, r.runtime, query, "group")
	result, err := query.Exec(ctx)
	if err != nil {
		return &domain.StoreError{Op: "update", Err: err}
	}
	if err := assertRowsAffected(result, 1, "update Group"); err != nil {
		return err
	}
	row.copyTo(group)
	return nil
}

// Delete removes a group of the caller's school.
func (r *GroupRepository) Delete(ctx context.Context, id any) error {
	query := r.runtime.DB(ctx).NewDelete().
		Model((*groupRow)(nil)).
		ModelTableExpr(groupTableExpr).
		Where(`"group".id = ?`, id)
	query = withTenantFilter(ctx, r.runtime, query, "group")
	if _, err := query.Exec(ctx); err != nil {
		return &domain.StoreError{Op: "delete", Err: err}
	}
	return nil
}

// FindByName retrieves a group by its name
func (r *GroupRepository) FindByName(ctx context.Context, name string) (*domain.Group, error) {
	row := new(groupRow)
	query := r.runtime.DB(ctx).NewSelect().
		Model(row).
		ModelTableExpr(groupTableExpr).
		Where("LOWER(name) = LOWER(?)", name)

	query = withTenantFilter(ctx, r.runtime, query, "group")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &domain.StoreError{
			Op:  "find by name",
			Err: translateNotFound(err),
		}
	}

	return row.group(), nil
}

// FindByIDs retrieves multiple groups by their IDs in a single query
func (r *GroupRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*domain.Group, error) {
	if len(ids) == 0 {
		return make(map[int64]*domain.Group), nil
	}

	var rows []*groupRow
	query := r.runtime.DB(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(groupTableExpr).
		Where(`"group".id IN (?)`, bun.List(ids))

	query = withTenantFilter(ctx, r.runtime, query, "group")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &domain.StoreError{
			Op:  "find by IDs",
			Err: translateNotFound(err),
		}
	}
	groups := groupsOf(rows)

	// Convert to map for O(1) lookups
	result := make(map[int64]*domain.Group, len(groups))
	for _, group := range groups {
		result[group.ID] = group
	}

	return result, nil
}

// FindByTeacher retrieves groups by their teacher ID (via group_teacher table)
func (r *GroupRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*domain.Group, error) {
	if r.assignments == nil {
		return nil, errors.New("group repository resolves teacher assignments through School Membership")
	}
	assignments, err := r.assignments(ctx, nil, []int64{teacherID})
	if err != nil {
		return nil, &domain.StoreError{
			Op:  "find by teacher",
			Err: err,
		}
	}
	ids := make([]int64, 0, len(assignments))
	for _, assignment := range assignments {
		ids = append(ids, assignment.GroupID)
	}
	byID, err := r.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	groups := make([]*domain.Group, 0, len(ids))
	for _, id := range ids {
		if group := byID[id]; group != nil {
			groups = append(groups, group)
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
// education groups: the teacher assignments, unresolved, plus the groups and
// day the substitutions must be read for. The composition layer turns them
// into (staff, group) pairs through School Membership and Workforce, dropping
// references to offboarded teachers and staff (#2667, #2688).
type GroupMembershipPairs struct {
	// Assigned pairs a group with the teacher assigned to it.
	Assigned []TeacherGroupID
	// GroupIDs and On name the substitutions to add: whoever substitutes in
	// one of these groups on that day. education.group_substitution belongs
	// to Workforce, so this repository does not read it.
	GroupIDs []int64
	On       calendar.Date
}

// listGroupMembershipPairs returns the unresolved supervision references of
// the given groups on the given day.
//
// The bulk mirror of usercontext.GetMyGroups read from the group side: the
// teacher assignments come from School Membership through the injected query,
// the substitutions active on the day from Workforce through the supervision
// resolver installed by the composition root.
func (r *GroupRepository) listGroupMembershipPairs(ctx context.Context, groupIDs []int64, on calendar.Date) (GroupMembershipPairs, error) {
	var pairs GroupMembershipPairs
	if len(groupIDs) == 0 {
		return pairs, nil
	}

	if r.assignments == nil {
		return GroupMembershipPairs{}, errors.New("group repository resolves teacher assignments through School Membership")
	}
	assignments, err := r.assignments(ctx, groupIDs, nil)
	if err != nil {
		return GroupMembershipPairs{}, &domain.StoreError{
			Op:  "list staff IDs by education group IDs (assigned)",
			Err: err,
		}
	}
	pairs.Assigned = append(pairs.Assigned, assignments...)
	pairs.GroupIDs = groupIDs
	pairs.On = on

	return pairs, nil
}

// ListStaffIDsByEducationGroupIDs returns the (staff, group) pairs
// supervising the given groups on the given day: teacher assignments plus
// substitutions active on that day, and nobody else. Resolving a teacher to
// their staff member, and dropping offboarded teachers and staff, is done by
// the injected School Membership lookup.
func (r *GroupRepository) ListStaffIDsByEducationGroupIDs(ctx context.Context, groupIDs []int64, on calendar.Date) ([]domain.StaffGroupID, error) {
	if len(groupIDs) == 0 {
		return []domain.StaffGroupID{}, nil
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

// FindWithRoom retrieves a group with its associated room
func (r *GroupRepository) FindWithRoom(ctx context.Context, groupID int64) (*domain.Group, error) {
	row := new(groupRow)
	query := r.runtime.DB(ctx).NewSelect().
		Model(row).
		ModelTableExpr(groupTableExpr).
		Where(`"group".id = ?`, groupID)

	query = withTenantFilter(ctx, r.runtime, query, "group")

	if err := query.Scan(ctx); err != nil {
		return nil, &domain.StoreError{
			Op:  "find with room",
			Err: translateNotFound(err),
		}
	}
	group := row.group()
	if err := attachRooms(ctx, r.roomDirectory(), []*domain.Group{group}); err != nil {
		return nil, &domain.StoreError{Op: "find with room", Err: err}
	}
	return group, nil
}

// FindByIDsWithRooms retrieves groups (keyed by ID) with their room relation
// preloaded via one LEFT JOIN — the bulk sibling of FindWithRoom, added so the
// OGS live projection resolves every supervised group's room name in a single
// query instead of one per group (#2094 review).
func (r *GroupRepository) FindByIDsWithRooms(ctx context.Context, ids []int64) (map[int64]*domain.Group, error) {
	result := make(map[int64]*domain.Group, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	var rows []*groupRow
	query := r.runtime.DB(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(groupTableExpr).
		Where(`"group".id IN (?)`, bun.List(ids))

	query = withTenantFilter(ctx, r.runtime, query, "group")

	if err := query.Scan(ctx); err != nil {
		return nil, &domain.StoreError{
			Op:  "find by IDs with rooms",
			Err: translateNotFound(err),
		}
	}
	groups := groupsOf(rows)
	if err := attachRooms(ctx, r.roomDirectory(), groups); err != nil {
		return nil, &domain.StoreError{Op: "find by IDs with rooms", Err: err}
	}

	for _, group := range groups {
		result[group.ID] = group
	}
	return result, nil
}

// List retrieves the groups of the caller's school matching the legacy map
// filters: "name_like" is a case-insensitive name match, "has_room" selects
// groups with or without a room, and any other key an equality on that column.
func (r *GroupRepository) List(ctx context.Context, filters map[string]any) ([]*domain.Group, error) {
	rows := make([]*groupRow, 0)
	query := r.runtime.DB(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(groupTableExpr)
	query = withTenantFilter(ctx, r.runtime, query, "group")
	for field, value := range filters {
		if value == nil {
			continue
		}
		query = applyGroupFilterField(query, field, value)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, &domain.StoreError{Op: "list with options", Err: err}
	}
	return groupsOf(rows), nil
}

// applyGroupFilterField applies a single legacy map filter.
func applyGroupFilterField(query *bun.SelectQuery, field string, value any) *bun.SelectQuery {
	switch field {
	case "name_like":
		if strValue, ok := value.(string); ok {
			return query.Where(`"group".name ILIKE ?`, "%"+strValue+"%")
		}
		return query
	case "has_room":
		if boolValue, ok := value.(bool); ok {
			if boolValue {
				return query.Where(`"group".room_id IS NOT NULL`)
			}
			return query.Where(`"group".room_id IS NULL`)
		}
		return query
	default:
		return query.Where("? = ?", bun.Ident("group."+field), value)
	}
}

// applyGroupListQuery applies the overview's filters, shared by the list and
// the count. Ordering and pagination remain list-only concerns.
func applyGroupListQuery(query *bun.SelectQuery, params *domain.GroupListQuery) *bun.SelectQuery {
	if params == nil {
		return query
	}
	if params.Name != "" {
		query = query.Where(`"group".name = ?`, params.Name)
	}
	if params.NameContains != "" {
		query = query.Where(`"group".name ILIKE ?`, "%"+params.NameContains+"%")
	}
	if params.RoomID != nil {
		query = query.Where(`"group".room_id = ?`, *params.RoomID)
	}
	return query
}

// ListWithRooms lists groups and their optional room in one snapshot: the
// groups from this owner, the rooms from Facilities (#2665).
func (r *GroupRepository) ListWithRooms(ctx context.Context, params *domain.GroupListQuery) ([]*domain.Group, error) {
	rows := make([]*groupRow, 0)
	query := r.runtime.DB(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(groupTableExpr)
	query = withTenantFilter(ctx, r.runtime, query, "group")
	query = applyGroupListQuery(query, params)
	if params != nil {
		if params.SortByName {
			if params.Descending {
				query = query.OrderExpr(`"group".name DESC`)
			} else {
				query = query.OrderExpr(`"group".name ASC`)
			}
		}
		if params.Limit > 0 {
			query = query.Limit(int64(params.Limit)).Offset(int64(params.Offset))
		}
	}
	if err := query.Scan(ctx); err != nil {
		return nil, &domain.StoreError{Op: "list with options", Err: translateNotFound(err)}
	}
	groups := groupsOf(rows)
	if err := attachRooms(ctx, r.roomDirectory(), groups); err != nil {
		return nil, &domain.StoreError{Op: "list with options", Err: err}
	}
	return groups, nil
}

// CountGroups counts the groups matching the overview's filters, ignoring
// ordering and pagination.
func (r *GroupRepository) CountGroups(ctx context.Context, params *domain.GroupListQuery) (int, error) {
	query := r.runtime.DB(ctx).NewSelect().
		Model((*groupRow)(nil)).
		ModelTableExpr(groupTableExpr).
		Column("group.id")
	query = withTenantFilter(ctx, r.runtime, query, "group")
	query = applyGroupListQuery(query, params)
	count, err := query.Count(ctx)
	if err != nil {
		return 0, &domain.StoreError{Op: "count with options", Err: err}
	}
	return int(count), nil
}

// Exists reports whether a group with the given ID exists in the current
// tenant (issue #584: moved verbatim from api/timetable template validation).
// Custom method (Rule 2): the generic shape has no EXISTS projection — going
// through List/Count would fetch or aggregate rows just to learn a boolean.
func (r *GroupRepository) Exists(ctx context.Context, id int64) (bool, error) {
	return r.runtime.DB(ctx).NewSelect().
		TableExpr(groupTableExpr).
		Where(`"group".tenant_id = ?`, r.runtime.TenantID(ctx)).
		Where(`"group".id = ?`, id).
		Exists(ctx)
}

// roomDirectory resolves the construction-time owner binding. The legacy
// root swaps its Facilities capability without exposing a provider setter.
func (r *GroupRepository) roomDirectory() RoomDirectory {
	if r.rooms == nil {
		return nil
	}
	return r.rooms()
}
