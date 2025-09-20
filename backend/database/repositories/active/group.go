// backend/database/repositories/active/group.go
package active

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/uptrace/bun"
)

// GroupRepository implements active.GroupRepository interface
type GroupRepository struct {
	*base.Repository[*active.Group]
	db *bun.DB
}

// NewGroupRepository creates a new GroupRepository
func NewGroupRepository(db *bun.DB) active.GroupRepository {
	return &GroupRepository{
		Repository: base.NewRepository[*active.Group](db, "active.groups", "Group"),
		db:         db,
	}
}

// FindActiveByRoomID finds all active groups in a specific room
func (r *GroupRepository) FindActiveByRoomID(ctx context.Context, roomID int64) ([]*active.Group, error) {
	var groups []*active.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Where("room_id = ? AND end_time IS NULL", roomID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by room ID",
			Err: err,
		}
	}

	return groups, nil
}

// FindActiveByGroupID finds all active instances of a specific activity group
func (r *GroupRepository) FindActiveByGroupID(ctx context.Context, groupID int64) ([]*active.Group, error) {
	var groups []*active.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Where("group_id = ? AND end_time IS NULL", groupID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by group ID",
			Err: err,
		}
	}

	return groups, nil
}

// FindByTimeRange finds all groups active during a specific time range
func (r *GroupRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Group, error) {
	var groups []*active.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Where("start_time <= ? AND (end_time IS NULL OR end_time >= ?)", end, start).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by time range",
			Err: err,
		}
	}

	return groups, nil
}

// EndSession marks a group session as ended at the current time
func (r *GroupRepository) EndSession(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Model((*active.Group)(nil)).
		ModelTableExpr(`active.groups AS "group"`).
		Set("end_time = ?", time.Now()).
		Where("id = ? AND end_time IS NULL", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "end session",
			Err: err,
		}
	}

	return nil
}

// Create overrides base Create to handle validation
func (r *GroupRepository) Create(ctx context.Context, group *active.Group) error {
	if group == nil {
		return fmt.Errorf("group cannot be nil")
	}

	// Validate group
	if err := group.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, group)
}

// List overrides the base List method to accept the new QueryOptions type
func (r *GroupRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.Group, error) {
	var groups []*active.Group
	query := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`)

	// Apply query options with table alias
	if options != nil {
		if options.Filter != nil {
			options.Filter.WithTableAlias("group")
		}
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return groups, nil
}

// FindWithRelations retrieves a group with its associated relations
func (r *GroupRepository) FindWithRelations(ctx context.Context, id int64) (*active.Group, error) {
	group := new(active.Group)
	err := r.db.NewSelect().
		Model(group).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".id = ?`, id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with relations",
			Err: err,
		}
	}

	return group, nil
}

// FindWithVisits retrieves a group with its associated visits
func (r *GroupRepository) FindWithVisits(ctx context.Context, id int64) (*active.Group, error) {
	group := new(active.Group)
	err := r.db.NewSelect().
		Model(group).
		ModelTableExpr(`active.groups AS "group"`).
		Relation("Visits").
		Where(`"group".id = ?`, id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with visits",
			Err: err,
		}
	}

	return group, nil
}

// FindWithSupervisors retrieves a group with its associated supervisors
func (r *GroupRepository) FindWithSupervisors(ctx context.Context, id int64) (*active.Group, error) {
	// First get the group
	group := new(active.Group)
	err := r.db.NewSelect().
		Model(group).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".id = ?`, id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with supervisors - group",
			Err: err,
		}
	}

	// Then get the supervisors
	var supervisors []*active.GroupSupervisor
	err = r.db.NewSelect().
		Model(&supervisors).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Where(`"group_supervisor".group_id = ?`, id).
		Scan(ctx)

	if err != nil {
		// Don't fail if no supervisors found
		if err != sql.ErrNoRows {
			return nil, &modelBase.DatabaseError{
				Op:  "find with supervisors - supervisors",
				Err: err,
			}
		}
	}

	group.Supervisors = supervisors
	return group, nil
}

// FindBySourceIDs finds active groups based on source IDs and source type
func (r *GroupRepository) FindBySourceIDs(ctx context.Context, sourceIDs []int64, sourceType string) ([]*active.Group, error) {
	if len(sourceIDs) == 0 {
		return []*active.Group{}, nil
	}

	var groups []*active.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Where("source_id IN (?) AND source_type = ? AND end_time IS NULL", bun.In(sourceIDs), sourceType).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by source IDs",
			Err: err,
		}
	}

	return groups, nil
}

// Activity session conflict detection methods

// FindActiveByGroupIDWithDevice finds all active instances of a specific activity group with device information
func (r *GroupRepository) FindActiveByGroupIDWithDevice(ctx context.Context, groupID int64) ([]*active.Group, error) {
	var groups []*active.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Where("group_id = ? AND end_time IS NULL", groupID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by group ID with device",
			Err: err,
		}
	}

	return groups, nil
}

// FindActiveByDeviceID finds the current active session for a specific device
func (r *GroupRepository) FindActiveByDeviceID(ctx context.Context, deviceID int64) (*active.Group, error) {
	type basicGroup struct {
		ID             int64      `bun:"id"`
		StartTime      time.Time  `bun:"start_time"`
		EndTime        *time.Time `bun:"end_time"`
		LastActivity   time.Time  `bun:"last_activity"`
		TimeoutMinutes int        `bun:"timeout_minutes"`
		GroupID        int64      `bun:"group_id"`
		DeviceID       *int64     `bun:"device_id"`
		RoomID         int64      `bun:"room_id"`
		CreatedAt      time.Time  `bun:"created_at"`
		UpdatedAt      time.Time  `bun:"updated_at"`
	}

	var result basicGroup
	err := r.db.NewSelect().
		TableExpr("active.groups AS ag").
		ColumnExpr("ag.id, ag.start_time, ag.end_time, ag.last_activity, ag.timeout_minutes").
		ColumnExpr("ag.group_id, ag.device_id, ag.room_id, ag.created_at, ag.updated_at").
		Where("ag.device_id = ? AND ag.end_time IS NULL", deviceID).
		Scan(ctx, &result)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil // No active session found - not an error
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find active by device ID",
			Err: err,
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

// FindActiveByDeviceIDWithRelations finds the current active session for a specific device with activity and room details
// DEPRECATED: Use FindActiveByDeviceIDWithNames instead to avoid BUN relation conflicts
func (r *GroupRepository) FindActiveByDeviceIDWithRelations(ctx context.Context, deviceID int64) (*active.Group, error) {
	// Redirect to the working method to avoid BUN schema conflicts
	return r.FindActiveByDeviceIDWithNames(ctx, deviceID)
}

// FindActiveByDeviceIDWithNames finds the current active session for a device with activity and room names using direct SQL
func (r *GroupRepository) FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*active.Group, error) {
	type sessionQueryResult struct {
		ID             int64      `bun:"id"`
		StartTime      time.Time  `bun:"start_time"`
		EndTime        *time.Time `bun:"end_time"`
		LastActivity   time.Time  `bun:"last_activity"`
		TimeoutMinutes int        `bun:"timeout_minutes"`
		GroupID        int64      `bun:"group_id"`
		DeviceID       *int64     `bun:"device_id"`
		RoomID         int64      `bun:"room_id"`
		CreatedAt      time.Time  `bun:"created_at"`
		UpdatedAt      time.Time  `bun:"updated_at"`
		ActivityName   *string    `bun:"activity_name"`
		RoomName       *string    `bun:"room_name"`
	}

	var result sessionQueryResult

	// Use facilities service pattern: TableExpr with explicit schema.table names
	// This avoids BUN model hooks that cause "groups does not exist" errors
	err := r.db.NewSelect().
		TableExpr("active.groups AS ag").
		ColumnExpr("ag.id, ag.start_time, ag.end_time, ag.last_activity, ag.timeout_minutes").
		ColumnExpr("ag.group_id, ag.device_id, ag.room_id, ag.created_at, ag.updated_at").
		ColumnExpr("actg.name AS activity_name"). // Use 'actg' not 'act' to avoid confusion
		ColumnExpr("rm.name AS room_name").       // Use 'rm' not 'r' for clarity
		Join("LEFT JOIN activities.groups AS actg ON actg.id = ag.group_id").
		Join("LEFT JOIN facilities.rooms AS rm ON rm.id = ag.room_id").
		Where("ag.device_id = ? AND ag.end_time IS NULL", deviceID).
		Scan(ctx, &result)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil // No active session found - not an error
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find active by device ID with names",
			Err: err,
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

	// Add activity info if available
	if result.ActivityName != nil && *result.ActivityName != "" {
		session.ActualGroup = &activities.Group{
			Model: modelBase.Model{ID: result.GroupID},
			Name:  *result.ActivityName,
		}
	}

	// Add room info if available
	if result.RoomName != nil && *result.RoomName != "" {
		session.Room = &facilities.Room{
			Model: modelBase.Model{ID: result.RoomID},
			Name:  *result.RoomName,
		}
	}

	return session, nil
}

// CheckActivityDeviceConflict checks if an activity is already running on another device
// CheckRoomConflict checks if a room is already occupied by another active group
func (r *GroupRepository) CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *active.Group, error) {
	var group active.Group
	query := r.db.NewSelect().
		Model(&group).
		ModelTableExpr(`active.groups AS "group"`).
		Where("room_id = ? AND end_time IS NULL", roomID)

	// Exclude the current group if specified (for updates)
	if excludeGroupID > 0 {
		query = query.Where("id != ?", excludeGroupID)
	}

	err := query.Scan(ctx)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return false, nil, nil // No conflict found
		}
		return false, nil, &modelBase.DatabaseError{
			Op:  "check room conflict",
			Err: err,
		}
	}

	// Conflict found
	return true, &group, nil
}

// UpdateLastActivity updates the last activity timestamp for a session
func (r *GroupRepository) UpdateLastActivity(ctx context.Context, id int64, lastActivity time.Time) error {
	// Use the base repository's transaction support
	query := r.db.NewUpdate().
		Model((*active.Group)(nil)).
		ModelTableExpr(`active.groups AS "group"`).
		Set("last_activity = ?", lastActivity).
		Set("updated_at = ?", time.Now()).
		Where("id = ? AND end_time IS NULL", id)

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update last activity",
			Err: err,
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update last activity - check rows affected",
			Err: err,
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
func (r *GroupRepository) FindActiveSessionsOlderThan(ctx context.Context, cutoffTime time.Time) ([]*active.Group, error) {
	var groups []*active.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Where("end_time IS NULL").              // Only active sessions
		Where("last_activity < ?", cutoffTime). // Haven't had activity since cutoff
		Where("device_id IS NOT NULL").         // Only device-managed sessions
		Order("last_activity ASC").             // Oldest first
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active sessions older than",
			Err: err,
		}
	}

	return groups, nil
}

// FindInactiveSessions finds sessions that have been inactive for the specified duration
func (r *GroupRepository) FindInactiveSessions(ctx context.Context, inactiveDuration time.Duration) ([]*active.Group, error) {
	cutoffTime := time.Now().Add(-inactiveDuration)

	var groups []*active.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Where("end_time IS NULL").              // Only active sessions
		Where("last_activity < ?", cutoffTime). // Inactive for specified duration
		Where("device_id IS NOT NULL").         // Only device-managed sessions
		Where("timeout_minutes > 0").           // Has timeout configured
		// Only include sessions where inactivity exceeds their configured timeout
		Where("EXTRACT(EPOCH FROM (NOW() - last_activity))/60 >= timeout_minutes").
		Order("last_activity ASC"). // Oldest first
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find inactive sessions",
			Err: err,
		}
	}

	return groups, nil
}
