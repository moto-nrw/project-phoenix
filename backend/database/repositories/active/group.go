// backend/database/repositories/active/group.go
package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Query constants to avoid duplication (SonarCloud S1192)
const (
	tableExprActiveGroupsAG = "active.groups AS ag"
	whereEndTimeIsNull      = "ag.end_time IS NULL"
)

// GroupRepository implements active.GroupRepository interface
type GroupRepository struct {
	*base.Repository[*active.Group]
	db    *bun.DB
	rooms RoomDirectory
}

// BindRoomDirectory installs the Facilities directory the group reads
// resolve rooms through (#2665).
func (r *GroupRepository) BindRoomDirectory(rooms RoomDirectory) {
	r.rooms = rooms
}

// NewGroupRepository creates a new GroupRepository
func NewGroupRepository(db *bun.DB) active.GroupRepository {
	repo := base.NewRepository[*active.Group](db, "active.groups", "Group")
	repo.TenantScoped = true
	return &GroupRepository{
		Repository: repo,
		db:         db,
	}
}

// FindActiveByRoomID finds all active groups in a specific room
func (r *GroupRepository) FindActiveByRoomID(ctx context.Context, roomID int64) ([]*active.Group, error) {
	var groups []*active.Group
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".room_id = ? AND "group".end_time IS NULL`, roomID)

	query = base.WithTenantFilter(ctx, query, "group")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by room ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return groups, nil
}

// FindActiveByRoomIDAndDeviceID finds the active group in a room for a specific device.
func (r *GroupRepository) FindActiveByRoomIDAndDeviceID(ctx context.Context, roomID int64, deviceID int64) (*active.Group, error) {
	group := new(active.Group)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(group).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".room_id = ?`, roomID).
		Where(`"group".device_id = ?`, deviceID).
		Where(`"group".end_time IS NULL`).
		Limit(1).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find active by room ID and device ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return group, nil
}

// FindActiveByGroupID finds all active instances of a specific activity group
func (r *GroupRepository) FindActiveByGroupID(ctx context.Context, groupID int64) ([]*active.Group, error) {
	var groups []*active.Group
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".group_id = ? AND "group".end_time IS NULL`, groupID)

	query = base.WithTenantFilter(ctx, query, "group")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by group ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return groups, nil
}

// FindActiveByGroupIDs finds all active groups for multiple group IDs in a single query
func (r *GroupRepository) FindActiveByGroupIDs(ctx context.Context, groupIDs []int64) ([]*active.Group, error) {
	if len(groupIDs) == 0 {
		return []*active.Group{}, nil
	}

	var groups []*active.Group
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".group_id IN (?) AND "group".end_time IS NULL`, bun.List(groupIDs))

	query = base.WithTenantFilter(ctx, query, "group")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by group IDs",
			Err: base.TranslateNotFound(err),
		}
	}

	return groups, nil
}

// FindByTimeRange finds all groups active during a specific time range
func (r *GroupRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Group, error) {
	var groups []*active.Group
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Where("start_time <= ? AND (end_time IS NULL OR end_time >= ?)", end, start)

	query = base.WithTenantFilter(ctx, query, "group")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by time range",
			Err: base.TranslateNotFound(err),
		}
	}

	return groups, nil
}

// EndSession marks a group session as ended at the current time
func (r *GroupRepository) EndSession(ctx context.Context, id int64) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*active.Group)(nil)).
		ModelTableExpr(`active.groups AS "group"`).
		Set("end_time = ?", time.Now()).
		Where(`"group".id = ? AND "group".end_time IS NULL`, id)

	query = base.WithTenantFilter(ctx, query, "group")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "end session",
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "end session")
}

// List overrides the base List method to accept the new QueryOptions type
func (r *GroupRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.Group, error) {
	return r.ListWithOptions(ctx, options)
}

// FindWithSupervisors retrieves a group with its associated supervisors
func (r *GroupRepository) FindWithSupervisors(ctx context.Context, id int64) (*active.Group, error) {
	// First get the group
	group := new(active.Group)
	groupQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(group).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".id = ?`, id)

	groupQuery = base.WithTenantFilter(ctx, groupQuery, "group")

	err := groupQuery.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with supervisors - group",
			Err: base.TranslateNotFound(err),
		}
	}

	// Then get the supervisors
	var supervisors []*active.GroupSupervisor
	supQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(&supervisors).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Where(`"group_supervisor".group_id = ?`, id)

	supQuery = base.WithTenantFilter(ctx, supQuery, "group_supervisor")

	err = supQuery.Scan(ctx)

	if err != nil {
		// Don't fail if no supervisors found
		if err != sql.ErrNoRows {
			return nil, &modelBase.DatabaseError{
				Op:  "find with supervisors - supervisors",
				Err: base.TranslateNotFound(err),
			}
		}
	}

	group.Supervisors = supervisors
	return group, nil
}

// Activity session conflict detection methods

// FindActiveByDeviceID finds the current active session for a specific device
func (r *GroupRepository) FindActiveByDeviceID(ctx context.Context, deviceID int64) (*active.Group, error) {
	type basicGroup struct {
		ID             int64      `bun:"id"`
		StartTime      time.Time  `bun:"start_time"`
		EndTime        *time.Time `bun:"end_time"`
		LastActivity   time.Time  `bun:"last_activity"`
		TimeoutMinutes int        `bun:"timeout_minutes"`
		GroupID        *int64     `bun:"group_id"`
		DeviceID       *int64     `bun:"device_id"`
		RoomID         int64      `bun:"room_id"`
		CreatedAt      time.Time  `bun:"created_at"`
		UpdatedAt      time.Time  `bun:"updated_at"`
	}

	var result basicGroup
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprActiveGroupsAG).
		ColumnExpr("ag.id, ag.start_time, ag.end_time, ag.last_activity, ag.timeout_minutes").
		ColumnExpr("ag.group_id, ag.device_id, ag.room_id, ag.created_at, ag.updated_at").
		Where("ag.device_id = ? AND ag.end_time IS NULL", deviceID)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("ag.tenant_id = ?", tenantID)
	}

	err := query.Scan(ctx, &result)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No active session found - not an error
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find active by device ID",
			Err: base.TranslateNotFound(err),
		}
	}

	// Convert to active.Group without relations
	group := &active.Group{
		Model: modelBase.Model{
			ID:        result.ID,
			CreatedAt: result.CreatedAt,
			UpdatedAt: result.UpdatedAt,
		},
		StartTime:      result.StartTime,
		EndTime:        result.EndTime,
		LastActivity:   result.LastActivity,
		TimeoutMinutes: result.TimeoutMinutes,
		GroupID:        result.GroupID,
		DeviceID:       result.DeviceID,
		RoomID:         result.RoomID,
	}

	return group, nil
}

// FindActiveByDeviceIDWithNames finds the current active session for a device with activity and room names using direct SQL
func (r *GroupRepository) FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*active.Group, error) {
	type sessionQueryResult struct {
		ID             int64      `bun:"id"`
		StartTime      time.Time  `bun:"start_time"`
		EndTime        *time.Time `bun:"end_time"`
		LastActivity   time.Time  `bun:"last_activity"`
		TimeoutMinutes int        `bun:"timeout_minutes"`
		GroupID        *int64     `bun:"group_id"`
		DeviceID       *int64     `bun:"device_id"`
		RoomID         int64      `bun:"room_id"`
		CreatedAt      time.Time  `bun:"created_at"`
		UpdatedAt      time.Time  `bun:"updated_at"`
		ActivityName   *string    `bun:"activity_name"`
	}

	var result sessionQueryResult

	// Use facilities service pattern: TableExpr with explicit schema.table names
	// This avoids BUN model hooks that cause "groups does not exist" errors
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprActiveGroupsAG).
		ColumnExpr("ag.id, ag.start_time, ag.end_time, ag.last_activity, ag.timeout_minutes").
		ColumnExpr("ag.group_id, ag.device_id, ag.room_id, ag.created_at, ag.updated_at").
		ColumnExpr("actg.name AS activity_name"). // Use 'actg' not 'act' to avoid confusion
		Join("LEFT JOIN activities.groups AS actg ON actg.id = ag.group_id").
		Where("ag.device_id = ? AND ag.end_time IS NULL", deviceID)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("ag.tenant_id = ?", tenantID)
	}

	err := query.Scan(ctx, &result)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No active session found - not an error
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find active by device ID with names",
			Err: base.TranslateNotFound(err),
		}
	}

	// Create active.Group from result
	session := &active.Group{
		Model: modelBase.Model{
			ID:        result.ID,
			CreatedAt: result.CreatedAt,
			UpdatedAt: result.UpdatedAt,
		},
		StartTime:      result.StartTime,
		EndTime:        result.EndTime,
		LastActivity:   result.LastActivity,
		TimeoutMinutes: result.TimeoutMinutes,
		GroupID:        result.GroupID,
		DeviceID:       result.DeviceID,
		RoomID:         result.RoomID,
	}

	// Add activity info if available. The LEFT JOIN only yields an ActivityName
	// when group_id is non-NULL and matches an activities.groups row, so a
	// non-empty name implies result.GroupID is non-nil. We still guard the
	// dereference explicitly to keep this robust against future query changes.
	if result.GroupID != nil && result.ActivityName != nil && *result.ActivityName != "" {
		session.ActualGroup = &activities.Group{
			Model: modelBase.Model{ID: *result.GroupID},
			Name:  *result.ActivityName,
		}
	}

	// Add room info if available: the room owner resolves the name the
	// former LEFT JOIN projected (#2665).
	rooms, err := roomsByID(ctx, r.rooms, []int64{result.RoomID})
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find active by device ID with names", Err: err}
	}
	if room, ok := rooms[result.RoomID]; ok && room.Name != "" {
		session.Room = &facilities.Room{
			ID:   result.RoomID,
			Name: room.Name,
		}
	}

	return session, nil
}

// CheckActivityDeviceConflict checks if an activity is already running on another device
// CheckRoomConflict checks if a room is already occupied by another active group
func (r *GroupRepository) CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *active.Group, error) {
	var group active.Group
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&group).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".room_id = ? AND "group".end_time IS NULL`, roomID)

	// Exclude the current group if specified (for updates)
	if excludeGroupID > 0 {
		query = query.Where(`"group".id != ?`, excludeGroupID)
	}

	query = base.WithTenantFilter(ctx, query, "group")

	err := query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil, nil // No conflict found
		}
		return false, nil, &modelBase.DatabaseError{
			Op:  "check room conflict",
			Err: base.TranslateNotFound(err),
		}
	}

	// Conflict found
	return true, &group, nil
}

// UpdateLastActivity updates the last activity timestamp for a session
func (r *GroupRepository) UpdateLastActivity(ctx context.Context, id int64, lastActivity time.Time) error {
	// Use the base repository's transaction support
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*active.Group)(nil)).
		ModelTableExpr(`active.groups AS "group"`).
		Set("last_activity = ?", lastActivity).
		Set("updated_at = ?", time.Now()).
		Where(`"group".id = ? AND "group".end_time IS NULL`, id)

	query = base.WithTenantFilter(ctx, query, "group")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update last activity",
			Err: base.TranslateNotFound(err),
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update last activity - check rows affected",
			Err: base.TranslateNotFound(err),
		}
	}

	if rowsAffected == 0 {
		return &modelBase.DatabaseError{
			Op:  "update last activity - session not found",
			Err: fmt.Errorf("active group with id %d not found or already ended", id),
		}
	}

	return nil
}

// FindActiveSessionsOlderThan finds active sessions that haven't had activity since the cutoff time
// Also loads the Device relation to check device online status
func (r *GroupRepository) FindActiveSessionsOlderThan(ctx context.Context, cutoffTime time.Time) ([]*active.Group, error) {
	// Query result struct to hold joined data
	type sessionWithDevice struct {
		ID             int64      `bun:"id"`
		CreatedAt      time.Time  `bun:"created_at"`
		UpdatedAt      time.Time  `bun:"updated_at"`
		TenantID       int64      `bun:"tenant_id"`
		StartTime      time.Time  `bun:"start_time"`
		EndTime        *time.Time `bun:"end_time"`
		LastActivity   time.Time  `bun:"last_activity"`
		TimeoutMinutes int        `bun:"timeout_minutes"`
		GroupID        *int64     `bun:"group_id"`
		DeviceID       *int64     `bun:"device_id"`
		RoomID         int64      `bun:"room_id"`
		// Device fields
		DeviceDbID       *int64     `bun:"device__id"`
		DeviceCreatedAt  *time.Time `bun:"device__created_at"`
		DeviceUpdatedAt  *time.Time `bun:"device__updated_at"`
		DeviceDeviceID   *string    `bun:"device__device_id"`
		DeviceDeviceType *string    `bun:"device__device_type"`
		DeviceName       *string    `bun:"device__name"`
		DeviceStatus     *string    `bun:"device__status"`
		DeviceLastSeen   *time.Time `bun:"device__last_seen"`
	}

	var results []sessionWithDevice

	// Use explicit JOIN with schema-qualified table name (BUN Relation() doesn't work with multi-schema)
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprActiveGroupsAG).
		ColumnExpr("ag.id, ag.created_at, ag.updated_at, ag.tenant_id, ag.start_time, ag.end_time").
		ColumnExpr("ag.last_activity, ag.timeout_minutes, ag.group_id, ag.device_id, ag.room_id").
		ColumnExpr(`d.id AS "device__id", d.created_at AS "device__created_at", d.updated_at AS "device__updated_at"`).
		ColumnExpr(`d.device_id AS "device__device_id", d.device_type AS "device__device_type"`).
		ColumnExpr(`d.name AS "device__name", d.status AS "device__status", d.last_seen AS "device__last_seen"`).
		Join("LEFT JOIN iot.devices AS d ON d.id = ag.device_id").
		Where(whereEndTimeIsNull).                 // Only active sessions
		Where("ag.last_activity < ?", cutoffTime). // Haven't had activity since cutoff
		Where("ag.device_id IS NOT NULL").         // Only device-managed sessions
		Order("ag.last_activity ASC")              // Oldest first

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("ag.tenant_id = ?", tenantID)
	}

	err := query.Scan(ctx, &results)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active sessions older than",
			Err: base.TranslateNotFound(err),
		}
	}

	// Convert results to active.Group with Device populated
	groups := make([]*active.Group, len(results))
	for i, r := range results {
		group := &active.Group{
			Model: modelBase.Model{
				ID:        r.ID,
				CreatedAt: r.CreatedAt,
				UpdatedAt: r.UpdatedAt,
			},
			TenantModel:    modelBase.TenantModel{TenantID: r.TenantID},
			StartTime:      r.StartTime,
			EndTime:        r.EndTime,
			LastActivity:   r.LastActivity,
			TimeoutMinutes: r.TimeoutMinutes,
			GroupID:        r.GroupID,
			DeviceID:       r.DeviceID,
			RoomID:         r.RoomID,
		}

		// Populate Device if present
		if r.DeviceDbID != nil {
			group.Device = &iot.Device{
				Model: modelBase.Model{
					ID:        *r.DeviceDbID,
					CreatedAt: *r.DeviceCreatedAt,
					UpdatedAt: *r.DeviceUpdatedAt,
				},
				DeviceID:   *r.DeviceDeviceID,
				DeviceType: *r.DeviceDeviceType,
				Name:       r.DeviceName,
				Status:     iot.DeviceStatus(*r.DeviceStatus),
				LastSeen:   r.DeviceLastSeen,
			}
		}

		groups[i] = group
	}

	return groups, nil
}

// FindActiveGroups finds all groups with no end time (currently active)
func (r *GroupRepository) FindActiveGroups(ctx context.Context) ([]*active.Group, error) {
	var groups []*active.Group
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".end_time IS NULL`)

	query = base.WithTenantFilter(ctx, query, "group")

	err := query.
		Order(`start_time ASC`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active groups",
			Err: base.TranslateNotFound(err),
		}
	}

	return groups, nil
}

// FindByIDs finds active groups by their IDs in a single query
func (r *GroupRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*active.Group, error) {
	if len(ids) == 0 {
		return make(map[int64]*active.Group), nil
	}

	uniqueIDs := sliceutil.Unique(ids)

	groups, err := r.queryGroupsByIDs(ctx, uniqueIDs)
	if err != nil {
		return nil, err
	}

	if err := r.loadRoomsForGroups(ctx, groups); err != nil {
		return nil, err
	}
	_, activityGroupIDs := collectRelationIDs(groups)
	if err := r.loadAndAssignActivityGroups(ctx, groups, activityGroupIDs); err != nil {
		return nil, err
	}

	return groupsToMap(groups), nil
}

// FindByIDForUpdate finds a group by ID and locks it for the current
// transaction. Returns nil when the row is not visible in the current tenant.
func (r *GroupRepository) FindByIDForUpdate(ctx context.Context, id int64) (*active.Group, error) {
	group := new(active.Group)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(group).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".id = ?`, id).
		For("UPDATE")

	query = base.WithTenantFilter(ctx, query, "group")

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find group by ID for update",
			Err: base.TranslateNotFound(err),
		}
	}
	return group, nil
}

// queryGroupsByIDs fetches groups by their IDs
func (r *GroupRepository) queryGroupsByIDs(ctx context.Context, ids []int64) ([]*active.Group, error) {
	var groups []*active.Group
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".id IN (?)`, bun.List(ids))

	query = base.WithTenantFilter(ctx, query, "group")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find groups by IDs", Err: base.TranslateNotFound(err)}
	}
	return groups, nil
}

// loadRoomsForGroups batch loads rooms for the given groups
func (r *GroupRepository) loadRoomsForGroups(ctx context.Context, groups []*active.Group) error {
	roomIDs := collectRoomIDs(groups)
	if len(roomIDs) == 0 {
		return nil
	}

	rooms, err := r.queryRoomsByIDs(ctx, roomIDs, "find group rooms by IDs")
	if err != nil {
		return err
	}

	assignRoomsToGroups(groups, rooms)
	return nil
}

// collectRoomIDs extracts unique room IDs from groups
func collectRoomIDs(groups []*active.Group) []int64 {
	seen := make(map[int64]struct{})
	ids := make([]int64, 0)
	for _, g := range groups {
		if g.RoomID > 0 {
			if _, exists := seen[g.RoomID]; !exists {
				seen[g.RoomID] = struct{}{}
				ids = append(ids, g.RoomID)
			}
		}
	}
	return ids
}

// queryRoomsByIDs fetches rooms by their IDs through the Facilities owner.
func (r *GroupRepository) queryRoomsByIDs(ctx context.Context, ids []int64, op string) ([]*facilities.Room, error) {
	found, err := roomsByID(ctx, r.rooms, ids)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: op, Err: err}
	}
	rooms := make([]*facilities.Room, 0, len(found))
	for _, room := range found {
		rooms = append(rooms, room.legacy())
	}
	return rooms, nil
}

// assignRoomsToGroups assigns rooms to groups based on room ID
func assignRoomsToGroups(groups []*active.Group, rooms []*facilities.Room) {
	roomMap := make(map[int64]*facilities.Room, len(rooms))
	for _, room := range rooms {
		roomMap[room.ID] = room
	}
	for _, g := range groups {
		if room, ok := roomMap[g.RoomID]; ok {
			g.Room = room
		}
	}
}

// groupsToMap converts a slice of groups to a map keyed by ID
func groupsToMap(groups []*active.Group) map[int64]*active.Group {
	result := make(map[int64]*active.Group, len(groups))
	for _, g := range groups {
		result[g.ID] = g
	}
	return result
}

// schulhofRoomName is the room the deviceless claim is limited to.
const schulhofRoomName = "Schulhof"

// FindUnclaimed finds all active groups that have no supervisors assigned
// This is used to allow teachers to claim Schulhof via the frontend
// Only returns groups in rooms named "Schulhof" - this is the only room that
// supports deviceless claiming. The room owner resolves the rooms; a group
// whose room is not visible is dropped, as the former INNER JOIN dropped it.
func (r *GroupRepository) FindUnclaimed(ctx context.Context) ([]*active.Group, error) {
	candidates, err := r.queryUnclaimedGroups(ctx)
	if err != nil {
		return nil, err
	}

	if err := r.loadUnclaimedGroupRelations(ctx, candidates); err != nil {
		return nil, err
	}

	groups := make([]*active.Group, 0, len(candidates))
	for _, g := range candidates {
		if g.Room != nil && g.Room.Name == schulhofRoomName {
			groups = append(groups, g)
		}
	}
	return groups, nil
}

// queryUnclaimedGroups fetches unclaimed groups from the database
func (r *GroupRepository) queryUnclaimedGroups(ctx context.Context) ([]*active.Group, error) {
	var groups []*active.Group
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Join(`LEFT JOIN active.group_supervisors AS "sup" ON "sup"."group_id" = "group"."id" AND ("sup"."end_date" IS NULL OR "sup"."end_date" > CURRENT_DATE)`).
		Where(`"group"."end_time" IS NULL`).
		Where(`"sup"."id" IS NULL`)

	query = base.WithTenantFilter(ctx, query, "group")

	err := query.
		Order("start_time DESC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find unclaimed groups", Err: base.TranslateNotFound(err)}
	}
	return groups, nil
}

// loadUnclaimedGroupRelations batch loads rooms and activity groups
func (r *GroupRepository) loadUnclaimedGroupRelations(ctx context.Context, groups []*active.Group) error {
	roomIDs, groupIDs := collectRelationIDs(groups)

	if err := r.loadAndAssignRooms(ctx, groups, roomIDs); err != nil {
		return err
	}

	if err := r.loadAndAssignActivityGroups(ctx, groups, groupIDs); err != nil {
		return err
	}

	return nil
}

// collectRelationIDs extracts unique room and group IDs. Spontaneous sessions
// (g.GroupID == nil) have no parent template — they are skipped here rather
// than materialising as spurious zero IDs.
func collectRelationIDs(groups []*active.Group) (roomIDs, groupIDs []int64) {
	roomSeen := make(map[int64]bool)
	groupSeen := make(map[int64]bool)

	for _, g := range groups {
		if g.RoomID > 0 && !roomSeen[g.RoomID] {
			roomIDs = append(roomIDs, g.RoomID)
			roomSeen[g.RoomID] = true
		}
		if templateID, ok := g.TemplateID(); ok && !groupSeen[templateID] {
			groupIDs = append(groupIDs, templateID)
			groupSeen[templateID] = true
		}
	}
	return roomIDs, groupIDs
}

// loadAndAssignRooms loads rooms and assigns them to groups
func (r *GroupRepository) loadAndAssignRooms(ctx context.Context, groups []*active.Group, roomIDs []int64) error {
	if len(roomIDs) == 0 {
		return nil
	}

	rooms, err := r.queryRoomsByIDs(ctx, roomIDs, "batch load rooms for unclaimed groups")
	if err != nil {
		return err
	}

	assignRoomsToGroups(groups, rooms)
	return nil
}

// loadAndAssignActivityGroups loads activity groups and assigns them
func (r *GroupRepository) loadAndAssignActivityGroups(ctx context.Context, groups []*active.Group, groupIDs []int64) error {
	if len(groupIDs) == 0 {
		return nil
	}

	activityGroups, err := r.queryActivityGroupsByIDs(ctx, groupIDs)
	if err != nil {
		return err
	}

	assignActivityGroupsToGroups(groups, activityGroups)
	return nil
}

// queryActivityGroupsByIDs fetches activity groups by their IDs
func (r *GroupRepository) queryActivityGroupsByIDs(ctx context.Context, ids []int64) ([]*activities.Group, error) {
	var groups []*activities.Group
	if err := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(`activities.groups AS "group"`).
		Where(`"group".id IN (?)`, bun.List(ids)).
		Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "batch load activity groups for unclaimed groups", Err: base.TranslateNotFound(err)}
	}
	return groups, nil
}

// assignActivityGroupsToGroups assigns activity groups to active groups
func assignActivityGroupsToGroups(groups []*active.Group, activityGroups []*activities.Group) {
	agMap := make(map[int64]*activities.Group, len(activityGroups))
	for _, ag := range activityGroups {
		agMap[ag.ID] = ag
	}
	for _, g := range groups {
		if templateID, ok := g.TemplateID(); ok {
			if ag, found := agMap[templateID]; found {
				g.ActualGroup = ag
			}
		}
	}
}

// GetOccupiedRoomIDs returns a set of room IDs that currently have active groups
// This is optimized for checking room occupancy without fetching full group records
func (r *GroupRepository) GetOccupiedRoomIDs(ctx context.Context, roomIDs []int64) (map[int64]bool, error) {
	if len(roomIDs) == 0 {
		return make(map[int64]bool), nil
	}

	// Only fetch the room_id column for active groups in the specified rooms
	var occupiedRoomIDs []int64
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprActiveGroupsAG).
		ColumnExpr("DISTINCT ag.room_id").
		Where("ag.room_id IN (?)", bun.List(roomIDs)).
		Where(whereEndTimeIsNull)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("ag.tenant_id = ?", tenantID)
	}

	err := query.Scan(ctx, &occupiedRoomIDs)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get occupied room IDs",
			Err: base.TranslateNotFound(err),
		}
	}

	// Convert to set for O(1) lookup
	result := make(map[int64]bool, len(occupiedRoomIDs))
	for _, id := range occupiedRoomIDs {
		result[id] = true
	}

	return result, nil
}

// EndSessionsByIDs ends multiple group sessions in a single query.
// Returns the number of sessions ended.
func (r *GroupRepository) EndSessionsByIDs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*active.Group)(nil)).
		ModelTableExpr(`active.groups AS "group"`).
		Set("end_time = ?", time.Now()).
		Where(`"group".id IN (?)`, bun.List(ids)).
		Where(`"group".end_time IS NULL`)

	query = base.WithTenantFilter(ctx, query, "group")

	result, err := query.Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "end sessions by IDs",
			Err: base.TranslateNotFound(err),
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "end sessions by IDs (rows affected)",
			Err: base.TranslateNotFound(err),
		}
	}

	return rowsAffected, nil
}

// GetOccupiedActivityGroupIDs returns a set of activity group IDs that currently have active sessions
// This is optimized for checking activity occupancy without fetching full group records
func (r *GroupRepository) GetOccupiedActivityGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]bool, error) {
	if len(groupIDs) == 0 {
		return make(map[int64]bool), nil
	}

	// Only fetch the group_id column for active groups with the specified activity group IDs
	var occupiedGroupIDs []int64
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprActiveGroupsAG).
		ColumnExpr("DISTINCT ag.group_id").
		Where("ag.group_id IN (?)", bun.List(groupIDs)).
		Where(whereEndTimeIsNull)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("ag.tenant_id = ?", tenantID)
	}

	err := query.Scan(ctx, &occupiedGroupIDs)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get occupied activity group IDs",
			Err: base.TranslateNotFound(err),
		}
	}

	// Convert to set for O(1) lookup
	result := make(map[int64]bool, len(occupiedGroupIDs))
	for _, id := range occupiedGroupIDs {
		result[id] = true
	}

	return result, nil
}

// AggregateRoomSessions builds the per-room occupancy timeline used by the
// room-history endpoint. One row per session: activity name,
// comma-separated supervisor names (staff, not students), distinct student
// count via correlated subquery, and computed duration in minutes. The
// student count follows the same window semantics as the session itself:
// only students whose visit overlapped [start, end] are counted. Result is
// ordered by session start time DESC. Tenant filters are applied
// explicitly on every joined table as defense-in-depth on top of RLS for
// normal request paths; the superuser / migration paths (tenantID == 0)
// bypass both RLS and these explicit filters by design.
//
// Window semantics: a session is included when it was *active during*
// [start, end] — i.e. it started before `end` AND either is still running
// (end_time IS NULL) or finished after `start`. Filtering on start_time
// alone would drop sessions that began before `start` but were still
// occupying the room inside the window, which is exactly the case the
// drawer wants to surface.
func (r *GroupRepository) AggregateRoomSessions(
	ctx context.Context,
	roomID int64,
	start, end time.Time,
	supervisorStaffID *int64,
) ([]*active.RoomSessionAggregate, error) {
	var rows []*active.RoomSessionAggregate

	tenantID := tenant.FromContext(ctx)

	// Each subquery is paired with its args via a local slice so a future
	// edit can't silently break the `?`-to-arg count. Tenant filters apply
	// only when a tenant is set; superuser / migration callers (tenantID
	// == 0) intentionally see everything — matches the rest of the repo
	// layer.
	supervisorSQL := `COALESCE((
		SELECT STRING_AGG(DISTINCT TRIM(CONCAT(p.first_name, ' ', p.last_name)), ', ' ORDER BY TRIM(CONCAT(p.first_name, ' ', p.last_name)))
		FROM active.group_supervisors gs
		JOIN users.staff s ON s.id = gs.staff_id
		JOIN users.persons p ON p.id = s.person_id
		WHERE gs.group_id = ag.id`
	supervisorArgs := []any{}
	if tenantID > 0 {
		supervisorSQL += ` AND gs.tenant_id = ? AND s.tenant_id = ? AND p.tenant_id = ?`
		supervisorArgs = append(supervisorArgs, tenantID, tenantID, tenantID)
	}
	supervisorSQL += `), '') AS supervisor_name`

	studentCountSQL := `COALESCE((
		SELECT COUNT(DISTINCT v.student_id)
		FROM active.visits v
		WHERE v.active_group_id = ag.id
		  AND v.entry_time <= ?
		  AND (v.exit_time IS NULL OR v.exit_time >= ?)`
	studentCountArgs := []any{end, start}
	if tenantID > 0 {
		studentCountSQL += ` AND v.tenant_id = ?`
		studentCountArgs = append(studentCountArgs, tenantID)
	}
	studentCountSQL += `), 0) AS student_count`

	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("active.groups AS ag").
		ColumnExpr("ag.id AS session_id").
		ColumnExpr("ag.start_time AS started_at").
		ColumnExpr("ag.end_time AS ended_at").
		ColumnExpr(`CASE WHEN ag.end_time IS NULL THEN NULL
			ELSE CAST(EXTRACT(EPOCH FROM (ag.end_time - ag.start_time)) / 60 AS INTEGER)
		END AS duration_minutes`).
		ColumnExpr(`COALESCE(g.name, '') AS activity_name`).
		Join("LEFT JOIN activities.groups g ON g.id = ag.group_id").
		Where("ag.room_id = ?", roomID).
		Where("ag.start_time <= ?", end).
		Where("(ag.end_time IS NULL OR ag.end_time >= ?)", start).
		OrderExpr("ag.start_time DESC")

	if len(supervisorArgs) > 0 {
		query = query.ColumnExpr(supervisorSQL, supervisorArgs...)
	} else {
		query = query.ColumnExpr(supervisorSQL)
	}
	if len(studentCountArgs) > 0 {
		query = query.ColumnExpr(studentCountSQL, studentCountArgs...)
	} else {
		query = query.ColumnExpr(studentCountSQL)
	}

	if tenantID > 0 {
		query = query.Where("ag.tenant_id = ?", tenantID)
	}

	if supervisorStaffID != nil {
		if tenantID > 0 {
			query = query.Where(`EXISTS (
				SELECT 1 FROM active.group_supervisors gs2
				WHERE gs2.group_id = ag.id
				  AND gs2.staff_id = ?
				  AND gs2.tenant_id = ?
			)`, *supervisorStaffID, tenantID)
		} else {
			query = query.Where(`EXISTS (
				SELECT 1 FROM active.group_supervisors gs2
				WHERE gs2.group_id = ag.id
				  AND gs2.staff_id = ?
			)`, *supervisorStaffID)
		}
	}

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "aggregate room sessions",
			Err: base.TranslateNotFound(err),
		}
	}

	return rows, nil
}
