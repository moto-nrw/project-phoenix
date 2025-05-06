package facilities

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// OccupancyStatus represents the status of a room occupancy
type OccupancyStatus string

const (
	OccupancyStatusActive      OccupancyStatus = "active"
	OccupancyStatusInactive    OccupancyStatus = "inactive"
	OccupancyStatusMaintenance OccupancyStatus = "maintenance"
)

// RoomOccupancy represents the current occupancy of a room
type RoomOccupancy struct {
	base.Model
	DeviceID         string          `bun:"device_id,unique" json:"device_id,omitempty"`
	RoomID           int64           `bun:"room_id,notnull" json:"room_id"`
	TimeframeID      int64           `bun:"timeframe_id,notnull" json:"timeframe_id"`
	Status           OccupancyStatus `bun:"status,notnull,default:'active'" json:"status"`
	MaxCapacity      int             `bun:"max_capacity,notnull,default:0" json:"max_capacity"`
	CurrentOccupancy int             `bun:"current_occupancy,notnull,default:0" json:"current_occupancy"`
	ActivityGroupID  *int64          `bun:"activity_group_id" json:"activity_group_id,omitempty"`
	GroupID          *int64          `bun:"group_id" json:"group_id,omitempty"`

	// Relations
	Room     *Room                   `bun:"rel:belongs-to,join:room_id=id" json:"room,omitempty"`
	Teachers []*RoomOccupancyTeacher `bun:"rel:has-many,join:id=room_occupancy_id" json:"teachers,omitempty"`
	Visits   []*Visit                `bun:"rel:has-many,join:id=room_occupancy_id" json:"visits,omitempty"`
}

// TableName returns the table name for the RoomOccupancy model
func (r *RoomOccupancy) TableName() string {
	return "facilities.room_occupancy"
}

// GetID returns the room occupancy ID
func (r *RoomOccupancy) GetID() interface{} {
	return r.ID
}

// GetCreatedAt returns the creation timestamp
func (r *RoomOccupancy) GetCreatedAt() time.Time {
	return r.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (r *RoomOccupancy) GetUpdatedAt() time.Time {
	return r.UpdatedAt
}

// Validate validates the room occupancy fields
func (r *RoomOccupancy) Validate() error {
	if r.RoomID <= 0 {
		return errors.New("room ID is required")
	}

	if r.TimeframeID <= 0 {
		return errors.New("timeframe ID is required")
	}

	if r.MaxCapacity < 0 {
		return errors.New("max capacity cannot be negative")
	}

	if r.CurrentOccupancy < 0 {
		return errors.New("current occupancy cannot be negative")
	}

	if r.CurrentOccupancy > r.MaxCapacity && r.MaxCapacity > 0 {
		return errors.New("current occupancy cannot exceed max capacity")
	}

	// Validate that only one of ActivityGroupID or GroupID is set
	if r.ActivityGroupID != nil && r.GroupID != nil {
		return errors.New("only one of activity group ID or group ID can be set")
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (r *RoomOccupancy) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := r.Model.BeforeAppend(); err != nil {
		return err
	}

	// Set default status if empty
	if r.Status == "" {
		r.Status = OccupancyStatusActive
	}

	return nil
}

// IsAvailable checks if the room occupancy is available
func (r *RoomOccupancy) IsAvailable() bool {
	return r.Status == OccupancyStatusActive && (r.MaxCapacity == 0 || r.CurrentOccupancy < r.MaxCapacity)
}

// RoomOccupancyRepository defines operations for working with room occupancies
type RoomOccupancyRepository interface {
	base.Repository[*RoomOccupancy]
	FindByRoom(ctx context.Context, roomID int64) ([]*RoomOccupancy, error)
	FindByTimeframe(ctx context.Context, timeframeID int64) ([]*RoomOccupancy, error)
	FindActiveByRoom(ctx context.Context, roomID int64) ([]*RoomOccupancy, error)
	UpdateStatus(ctx context.Context, id int64, status OccupancyStatus) error
	UpdateOccupancy(ctx context.Context, id int64, currentOccupancy int) error
	FindByDeviceID(ctx context.Context, deviceID string) (*RoomOccupancy, error)
	FindByActivity(ctx context.Context, activityGroupID int64) ([]*RoomOccupancy, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*RoomOccupancy, error)
}

// DefaultRoomOccupancyRepository is the default implementation of RoomOccupancyRepository
type DefaultRoomOccupancyRepository struct {
	db *bun.DB
}

// NewRoomOccupancyRepository creates a new room occupancy repository
func NewRoomOccupancyRepository(db *bun.DB) RoomOccupancyRepository {
	return &DefaultRoomOccupancyRepository{db: db}
}

// Create inserts a new room occupancy into the database
func (r *DefaultRoomOccupancyRepository) Create(ctx context.Context, roomOccupancy *RoomOccupancy) error {
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
func (r *DefaultRoomOccupancyRepository) FindByID(ctx context.Context, id interface{}) (*RoomOccupancy, error) {
	roomOccupancy := new(RoomOccupancy)
	err := r.db.NewSelect().Model(roomOccupancy).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return roomOccupancy, nil
}

// FindByRoom retrieves all room occupancies for a room
func (r *DefaultRoomOccupancyRepository) FindByRoom(ctx context.Context, roomID int64) ([]*RoomOccupancy, error) {
	var roomOccupancies []*RoomOccupancy
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
func (r *DefaultRoomOccupancyRepository) FindByTimeframe(ctx context.Context, timeframeID int64) ([]*RoomOccupancy, error) {
	var roomOccupancies []*RoomOccupancy
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
func (r *DefaultRoomOccupancyRepository) FindActiveByRoom(ctx context.Context, roomID int64) ([]*RoomOccupancy, error) {
	var roomOccupancies []*RoomOccupancy
	err := r.db.NewSelect().
		Model(&roomOccupancies).
		Where("room_id = ?", roomID).
		Where("status = ?", OccupancyStatusActive).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active_by_room", Err: err}
	}
	return roomOccupancies, nil
}

// FindByDeviceID retrieves a room occupancy by device ID
func (r *DefaultRoomOccupancyRepository) FindByDeviceID(ctx context.Context, deviceID string) (*RoomOccupancy, error) {
	roomOccupancy := new(RoomOccupancy)
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
func (r *DefaultRoomOccupancyRepository) FindByActivity(ctx context.Context, activityGroupID int64) ([]*RoomOccupancy, error) {
	var roomOccupancies []*RoomOccupancy
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
func (r *DefaultRoomOccupancyRepository) FindByGroup(ctx context.Context, groupID int64) ([]*RoomOccupancy, error) {
	var roomOccupancies []*RoomOccupancy
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
func (r *DefaultRoomOccupancyRepository) UpdateStatus(ctx context.Context, id int64, status OccupancyStatus) error {
	_, err := r.db.NewUpdate().
		Model((*RoomOccupancy)(nil)).
		Set("status = ?", status).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_status", Err: err}
	}
	return nil
}

// UpdateOccupancy updates the current occupancy of a room
func (r *DefaultRoomOccupancyRepository) UpdateOccupancy(ctx context.Context, id int64, currentOccupancy int) error {
	if currentOccupancy < 0 {
		return errors.New("current occupancy cannot be negative")
	}

	_, err := r.db.NewUpdate().
		Model((*RoomOccupancy)(nil)).
		Set("current_occupancy = ?", currentOccupancy).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_occupancy", Err: err}
	}
	return nil
}

// Update updates an existing room occupancy
func (r *DefaultRoomOccupancyRepository) Update(ctx context.Context, roomOccupancy *RoomOccupancy) error {
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
func (r *DefaultRoomOccupancyRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*RoomOccupancy)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves room occupancies matching the filters
func (r *DefaultRoomOccupancyRepository) List(ctx context.Context, filters map[string]interface{}) ([]*RoomOccupancy, error) {
	var roomOccupancies []*RoomOccupancy
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
