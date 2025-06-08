// backend/database/repositories/active/group.go
package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
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
		Relation("ActualGroup").
		Relation("Device").
		Relation("Room").
		Where("id = ?", id).
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
		Relation("Visits").
		Where("id = ?", id).
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
	group := new(active.Group)
	err := r.db.NewSelect().
		Model(group).
		Relation("Supervisors").
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with supervisors",
			Err: err,
		}
	}

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
	var group active.Group
	err := r.db.NewSelect().
		Model(&group).
		Where("device_id = ? AND end_time IS NULL", deviceID).
		Scan(ctx)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil // No active session found - not an error
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find active by device ID",
			Err: err,
		}
	}

	return &group, nil
}

// CheckActivityDeviceConflict checks if an activity is already running on another device
func (r *GroupRepository) CheckActivityDeviceConflict(ctx context.Context, activityID, excludeDeviceID int64) (bool, *active.Group, error) {
	var group active.Group
	query := r.db.NewSelect().
		Model(&group).
		Where("group_id = ? AND end_time IS NULL", activityID)

	// Exclude the requesting device if specified
	if excludeDeviceID > 0 {
		query = query.Where("device_id != ? OR device_id IS NULL", excludeDeviceID)
	}

	err := query.Scan(ctx)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return false, nil, nil // No conflict found
		}
		return false, nil, &modelBase.DatabaseError{
			Op:  "check activity device conflict",
			Err: err,
		}
	}

	// Conflict found
	return true, &group, nil
}