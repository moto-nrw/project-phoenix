package facilities

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// RoomHistory represents a historical record of room usage
type RoomHistory struct {
	base.Model
	RoomID            *int64    `bun:"room_id" json:"room_id,omitempty"`
	ActivityGroupName string    `bun:"activity_group_name,notnull" json:"activity_group_name"`
	Day               time.Time `bun:"day,notnull" json:"day"`
	TimeframeID       int64     `bun:"timeframe_id,notnull" json:"timeframe_id"`
	CategoryID        *int64    `bun:"category_id" json:"category_id,omitempty"`
	TeacherID         int64     `bun:"teacher_id,notnull" json:"teacher_id"`
	MaxParticipants   int       `bun:"max_participants,notnull,default:0" json:"max_participants"`
	GroupID           *int64    `bun:"group_id" json:"group_id,omitempty"`

	// Relations
	Room    *Room          `bun:"rel:belongs-to,join:room_id=id" json:"room,omitempty"`
	Teacher *users.Teacher `bun:"rel:belongs-to,join:teacher_id=id" json:"teacher,omitempty"`
}

// TableName returns the table name for the RoomHistory model
func (r *RoomHistory) TableName() string {
	return "facilities.room_history"
}

// GetID returns the room history ID
func (r *RoomHistory) GetID() interface{} {
	return r.ID
}

// GetCreatedAt returns the creation timestamp
func (r *RoomHistory) GetCreatedAt() time.Time {
	return r.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (r *RoomHistory) GetUpdatedAt() time.Time {
	return r.CreatedAt // This model only has created_at, no updated_at
}

// Validate validates the room history fields
func (r *RoomHistory) Validate() error {
	if strings.TrimSpace(r.ActivityGroupName) == "" {
		return errors.New("activity group name is required")
	}

	if r.Day.IsZero() {
		return errors.New("day is required")
	}

	if r.TimeframeID <= 0 {
		return errors.New("timeframe ID is required")
	}

	if r.TeacherID <= 0 {
		return errors.New("teacher ID is required")
	}

	if r.MaxParticipants < 0 {
		return errors.New("max participants cannot be negative")
	}

	// Check that only one of CategoryID or GroupID is set, or neither
	if r.CategoryID != nil && r.GroupID != nil {
		return errors.New("only one of category ID or group ID can be set")
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (r *RoomHistory) BeforeAppend() error {
	// No need to call parent's BeforeAppend as this model only has created_at

	// Trim whitespace
	r.ActivityGroupName = strings.TrimSpace(r.ActivityGroupName)

	return nil
}

// RoomHistoryRepository defines operations for working with room history
type RoomHistoryRepository interface {
	base.Repository[*RoomHistory]
	FindByRoom(ctx context.Context, roomID int64) ([]*RoomHistory, error)
	FindByDay(ctx context.Context, day time.Time) ([]*RoomHistory, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*RoomHistory, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*RoomHistory, error)
	FindByCategory(ctx context.Context, categoryID int64) ([]*RoomHistory, error)
	FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*RoomHistory, error)
	FindByActivityName(ctx context.Context, activityName string) ([]*RoomHistory, error)
}

// DefaultRoomHistoryRepository is the default implementation of RoomHistoryRepository
type DefaultRoomHistoryRepository struct {
	db *bun.DB
}

// NewRoomHistoryRepository creates a new room history repository
func NewRoomHistoryRepository(db *bun.DB) RoomHistoryRepository {
	return &DefaultRoomHistoryRepository{db: db}
}

// Create inserts a new room history into the database
func (r *DefaultRoomHistoryRepository) Create(ctx context.Context, roomHistory *RoomHistory) error {
	if err := roomHistory.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(roomHistory).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a room history by its ID
func (r *DefaultRoomHistoryRepository) FindByID(ctx context.Context, id interface{}) (*RoomHistory, error) {
	roomHistory := new(RoomHistory)
	err := r.db.NewSelect().Model(roomHistory).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return roomHistory, nil
}

// FindByRoom retrieves all room history for a room
func (r *DefaultRoomHistoryRepository) FindByRoom(ctx context.Context, roomID int64) ([]*RoomHistory, error) {
	var roomHistories []*RoomHistory
	err := r.db.NewSelect().
		Model(&roomHistories).
		Where("room_id = ?", roomID).
		Order("day DESC, timeframe_id ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_room", Err: err}
	}
	return roomHistories, nil
}

// FindByDay retrieves all room history for a day
func (r *DefaultRoomHistoryRepository) FindByDay(ctx context.Context, day time.Time) ([]*RoomHistory, error) {
	var roomHistories []*RoomHistory
	err := r.db.NewSelect().
		Model(&roomHistories).
		Where("day = ?", day).
		Order("room_id ASC, timeframe_id ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_day", Err: err}
	}
	return roomHistories, nil
}

// FindByTeacher retrieves all room history for a teacher
func (r *DefaultRoomHistoryRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*RoomHistory, error) {
	var roomHistories []*RoomHistory
	err := r.db.NewSelect().
		Model(&roomHistories).
		Where("teacher_id = ?", teacherID).
		Order("day DESC, timeframe_id ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_teacher", Err: err}
	}
	return roomHistories, nil
}

// FindByGroup retrieves all room history for a group
func (r *DefaultRoomHistoryRepository) FindByGroup(ctx context.Context, groupID int64) ([]*RoomHistory, error) {
	var roomHistories []*RoomHistory
	err := r.db.NewSelect().
		Model(&roomHistories).
		Where("group_id = ?", groupID).
		Order("day DESC, timeframe_id ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_group", Err: err}
	}
	return roomHistories, nil
}

// FindByCategory retrieves all room history for a category
func (r *DefaultRoomHistoryRepository) FindByCategory(ctx context.Context, categoryID int64) ([]*RoomHistory, error) {
	var roomHistories []*RoomHistory
	err := r.db.NewSelect().
		Model(&roomHistories).
		Where("category_id = ?", categoryID).
		Order("day DESC, timeframe_id ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_category", Err: err}
	}
	return roomHistories, nil
}

// FindByDateRange retrieves all room history within a date range
func (r *DefaultRoomHistoryRepository) FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*RoomHistory, error) {
	var roomHistories []*RoomHistory
	err := r.db.NewSelect().
		Model(&roomHistories).
		Where("day BETWEEN ? AND ?", startDate, endDate).
		Order("day ASC, room_id ASC, timeframe_id ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_date_range", Err: err}
	}
	return roomHistories, nil
}

// FindByActivityName retrieves all room history for an activity name
func (r *DefaultRoomHistoryRepository) FindByActivityName(ctx context.Context, activityName string) ([]*RoomHistory, error) {
	var roomHistories []*RoomHistory
	err := r.db.NewSelect().
		Model(&roomHistories).
		Where("activity_group_name ILIKE ?", "%"+activityName+"%").
		Order("day DESC, timeframe_id ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_activity_name", Err: err}
	}
	return roomHistories, nil
}

// Update updates an existing room history
func (r *DefaultRoomHistoryRepository) Update(ctx context.Context, roomHistory *RoomHistory) error {
	if err := roomHistory.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(roomHistory).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a room history
func (r *DefaultRoomHistoryRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*RoomHistory)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves room histories matching the filters
func (r *DefaultRoomHistoryRepository) List(ctx context.Context, filters map[string]interface{}) ([]*RoomHistory, error) {
	var roomHistories []*RoomHistory
	query := r.db.NewSelect().Model(&roomHistories)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return roomHistories, nil
}
