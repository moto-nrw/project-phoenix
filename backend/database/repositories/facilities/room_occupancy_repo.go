package facilities

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/uptrace/bun"
)

// RoomOccupancyRepository implements facilities.RoomOccupancyRepository
type RoomOccupancyRepository struct {
	db *bun.DB
}

// NewRoomOccupancyRepository creates a new room occupancy repository
func NewRoomOccupancyRepository(db *bun.DB) facilities.RoomOccupancyRepository {
	return &RoomOccupancyRepository{db: db}
}

// Create inserts a new room occupancy into the database
func (r *RoomOccupancyRepository) Create(ctx context.Context, roomOccupancy *facilities.RoomOccupancy) error {
	if err := roomOccupancy.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(roomOccupancy).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a room occupancy by its ID
func (r *RoomOccupancyRepository) FindByID(ctx context.Context, id interface{}) (*facilities.RoomOccupancy, error) {
	roomOccupancy := new(facilities.RoomOccupancy)
	err := r.db.NewSelect().Model(roomOccupancy).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return roomOccupancy, nil
}

// FindByRoom retrieves all room occupancies for a room
func (r *RoomOccupancyRepository) FindByRoom(ctx context.Context, roomID int64) ([]*facilities.RoomOccupancy, error) {
	var roomOccupancies []*facilities.RoomOccupancy
	err := r.db.NewSelect().
		Model(&roomOccupancies).
		Where("room_id = ?", roomID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_room", Err: err}
	}
	return roomOccupancies, nil
}

// FindByTimeframe retrieves all room occupancies for a timeframe
func (r *RoomOccupancyRepository) FindByTimeframe(ctx context.Context, timeframeID int64) ([]*facilities.RoomOccupancy, error) {
	var roomOccupancies []*facilities.RoomOccupancy
	err := r.db.NewSelect().
		Model(&roomOccupancies).
		Where("timeframe_id = ?", timeframeID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_timeframe", Err: err}
	}
	return roomOccupancies, nil
}

// FindActiveByRoom retrieves all active room occupancies for a room
func (r *RoomOccupancyRepository) FindActiveByRoom(ctx context.Context, roomID int64) ([]*facilities.RoomOccupancy, error) {
	var roomOccupancies []*facilities.RoomOccupancy
	err := r.db.NewSelect().
		Model(&roomOccupancies).
		Where("room_id = ?", roomID).
		Where("status = ?", facilities.OccupancyStatusActive).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active_by_room", Err: err}
	}
	return roomOccupancies, nil
}

// FindByDeviceID retrieves a room occupancy by device ID
func (r *RoomOccupancyRepository) FindByDeviceID(ctx context.Context, deviceID string) (*facilities.RoomOccupancy, error) {
	roomOccupancy := new(facilities.RoomOccupancy)
	err := r.db.NewSelect().
		Model(roomOccupancy).
		Where("device_id = ?", deviceID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_device_id", Err: err}
	}
	return roomOccupancy, nil
}

// FindByActivity retrieves all room occupancies for an activity group
func (r *RoomOccupancyRepository) FindByActivity(ctx context.Context, activityGroupID int64) ([]*facilities.RoomOccupancy, error) {
	var roomOccupancies []*facilities.RoomOccupancy
	err := r.db.NewSelect().
		Model(&roomOccupancies).
		Where("activity_group_id = ?", activityGroupID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_activity", Err: err}
	}
	return roomOccupancies, nil
}

// FindByGroup retrieves all room occupancies for a group
func (r *RoomOccupancyRepository) FindByGroup(ctx context.Context, groupID int64) ([]*facilities.RoomOccupancy, error) {
	var roomOccupancies []*facilities.RoomOccupancy
	err := r.db.NewSelect().
		Model(&roomOccupancies).
		Where("group_id = ?", groupID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_group", Err: err}
	}
	return roomOccupancies, nil
}

// UpdateStatus updates the status of a room occupancy
func (r *RoomOccupancyRepository) UpdateStatus(ctx context.Context, id int64, status facilities.OccupancyStatus) error {
	_, err := r.db.NewUpdate().
		Model((*facilities.RoomOccupancy)(nil)).
		Set("status = ?", status).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_status", Err: err}
	}
	return nil
}

// UpdateOccupancy updates the current occupancy of a room
func (r *RoomOccupancyRepository) UpdateOccupancy(ctx context.Context, id int64, currentOccupancy int) error {
	if currentOccupancy < 0 {
		return errors.New("current occupancy cannot be negative")
	}

	_, err := r.db.NewUpdate().
		Model((*facilities.RoomOccupancy)(nil)).
		Set("current_occupancy = ?", currentOccupancy).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_occupancy", Err: err}
	}
	return nil
}

// Update updates an existing room occupancy
func (r *RoomOccupancyRepository) Update(ctx context.Context, roomOccupancy *facilities.RoomOccupancy) error {
	if err := roomOccupancy.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(roomOccupancy).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a room occupancy
func (r *RoomOccupancyRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*facilities.RoomOccupancy)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves room occupancies matching the filters
func (r *RoomOccupancyRepository) List(ctx context.Context, filters map[string]interface{}) ([]*facilities.RoomOccupancy, error) {
	var roomOccupancies []*facilities.RoomOccupancy
	query := r.db.NewSelect().Model(&roomOccupancies)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return roomOccupancies, nil
}
