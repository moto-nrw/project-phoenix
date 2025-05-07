package facilities

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/uptrace/bun"
)

// RoomHistoryRepository implements facilities.RoomHistoryRepository
type RoomHistoryRepository struct {
	db *bun.DB
}

// NewRoomHistoryRepository creates a new room history repository
func NewRoomHistoryRepository(db *bun.DB) facilities.RoomHistoryRepository {
	return &RoomHistoryRepository{db: db}
}

// Create inserts a new room history into the database
func (r *RoomHistoryRepository) Create(ctx context.Context, roomHistory *facilities.RoomHistory) error {
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
func (r *RoomHistoryRepository) FindByID(ctx context.Context, id interface{}) (*facilities.RoomHistory, error) {
	roomHistory := new(facilities.RoomHistory)
	err := r.db.NewSelect().Model(roomHistory).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return roomHistory, nil
}

// FindByRoom retrieves all room history for a room
func (r *RoomHistoryRepository) FindByRoom(ctx context.Context, roomID int64) ([]*facilities.RoomHistory, error) {
	var roomHistories []*facilities.RoomHistory
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
func (r *RoomHistoryRepository) FindByDay(ctx context.Context, day time.Time) ([]*facilities.RoomHistory, error) {
	var roomHistories []*facilities.RoomHistory
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
func (r *RoomHistoryRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*facilities.RoomHistory, error) {
	var roomHistories []*facilities.RoomHistory
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
func (r *RoomHistoryRepository) FindByGroup(ctx context.Context, groupID int64) ([]*facilities.RoomHistory, error) {
	var roomHistories []*facilities.RoomHistory
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
func (r *RoomHistoryRepository) FindByCategory(ctx context.Context, categoryID int64) ([]*facilities.RoomHistory, error) {
	var roomHistories []*facilities.RoomHistory
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
func (r *RoomHistoryRepository) FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*facilities.RoomHistory, error) {
	var roomHistories []*facilities.RoomHistory
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
func (r *RoomHistoryRepository) FindByActivityName(ctx context.Context, activityName string) ([]*facilities.RoomHistory, error) {
	var roomHistories []*facilities.RoomHistory
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
func (r *RoomHistoryRepository) Update(ctx context.Context, roomHistory *facilities.RoomHistory) error {
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
func (r *RoomHistoryRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*facilities.RoomHistory)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves room histories matching the filters
func (r *RoomHistoryRepository) List(ctx context.Context, filters map[string]interface{}) ([]*facilities.RoomHistory, error) {
	var roomHistories []*facilities.RoomHistory
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
