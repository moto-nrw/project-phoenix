// backend/database/repositories/active/group_session.go
package active

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"
)

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
		TableExpr(tableExprActiveGroupsAG).
		ColumnExpr("ag.id, ag.start_time, ag.end_time, ag.last_activity, ag.timeout_minutes").
		ColumnExpr("ag.group_id, ag.device_id, ag.room_id, ag.created_at, ag.updated_at").
		Where("ag.device_id = ? AND ag.end_time IS NULL", deviceID).
		Scan(ctx, &result)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
		TableExpr(tableExprActiveGroupsAG).
		ColumnExpr("ag.id, ag.start_time, ag.end_time, ag.last_activity, ag.timeout_minutes").
		ColumnExpr("ag.group_id, ag.device_id, ag.room_id, ag.created_at, ag.updated_at").
		ColumnExpr("actg.name AS activity_name"). // Use 'actg' not 'act' to avoid confusion
		ColumnExpr("rm.name AS room_name").       // Use 'rm' not 'r' for clarity
		Join("LEFT JOIN activities.groups AS actg ON actg.id = ag.group_id").
		Join("LEFT JOIN facilities.rooms AS rm ON rm.id = ag.room_id").
		Where("ag.device_id = ? AND ag.end_time IS NULL", deviceID).
		Scan(ctx, &result)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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

// CheckRoomConflict checks if a room is already occupied by another active group
func (r *GroupRepository) CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *active.Group, error) {
	var group active.Group
	query := r.db.NewSelect().
		Model(&group).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".room_id = ? AND "group".end_time IS NULL`, roomID)

	// Exclude the current group if specified (for updates)
	if excludeGroupID > 0 {
		query = query.Where(`"group".id != ?`, excludeGroupID)
	}

	err := query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
		Where(`"group".id = ? AND "group".end_time IS NULL`, id)

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
// Also loads the Device relation to check device online status
func (r *GroupRepository) FindActiveSessionsOlderThan(ctx context.Context, cutoffTime time.Time) ([]*active.Group, error) {
	// Query result struct to hold joined data
	type sessionWithDevice struct {
		ID             int64      `bun:"id"`
		CreatedAt      time.Time  `bun:"created_at"`
		UpdatedAt      time.Time  `bun:"updated_at"`
		StartTime      time.Time  `bun:"start_time"`
		EndTime        *time.Time `bun:"end_time"`
		LastActivity   time.Time  `bun:"last_activity"`
		TimeoutMinutes int        `bun:"timeout_minutes"`
		GroupID        int64      `bun:"group_id"`
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
	err := r.db.NewSelect().
		TableExpr(tableExprActiveGroupsAG).
		ColumnExpr("ag.id, ag.created_at, ag.updated_at, ag.start_time, ag.end_time").
		ColumnExpr("ag.last_activity, ag.timeout_minutes, ag.group_id, ag.device_id, ag.room_id").
		ColumnExpr(`d.id AS "device__id", d.created_at AS "device__created_at", d.updated_at AS "device__updated_at"`).
		ColumnExpr(`d.device_id AS "device__device_id", d.device_type AS "device__device_type"`).
		ColumnExpr(`d.name AS "device__name", d.status AS "device__status", d.last_seen AS "device__last_seen"`).
		Join("LEFT JOIN iot.devices AS d ON d.id = ag.device_id").
		Where("ag.end_time IS NULL").              // Only active sessions
		Where("ag.last_activity < ?", cutoffTime). // Haven't had activity since cutoff
		Where("ag.device_id IS NOT NULL").         // Only device-managed sessions
		Order("ag.last_activity ASC").             // Oldest first
		Scan(ctx, &results)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active sessions older than",
			Err: err,
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

// FindActiveGroups finds all groups with no end time (currently active)
func (r *GroupRepository) FindActiveGroups(ctx context.Context) ([]*active.Group, error) {
	var groups []*active.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".end_time IS NULL`).
		Order(`start_time ASC`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active groups",
			Err: err,
		}
	}

	return groups, nil
}
