package facilities

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/uptrace/bun"
)

// RoomOccupancyTeacherRepository implements facilities.RoomOccupancyTeacherRepository
type RoomOccupancyTeacherRepository struct {
	db *bun.DB
}

// NewRoomOccupancyTeacherRepository creates a new room occupancy teacher repository
func NewRoomOccupancyTeacherRepository(db *bun.DB) facilities.RoomOccupancyTeacherRepository {
	return &RoomOccupancyTeacherRepository{db: db}
}

// Create inserts a new room occupancy teacher into the database
func (r *RoomOccupancyTeacherRepository) Create(ctx context.Context, roomOccupancyTeacher *facilities.RoomOccupancyTeacher) error {
	if err := roomOccupancyTeacher.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(roomOccupancyTeacher).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a room occupancy teacher by its ID
func (r *RoomOccupancyTeacherRepository) FindByID(ctx context.Context, id interface{}) (*facilities.RoomOccupancyTeacher, error) {
	roomOccupancyTeacher := new(facilities.RoomOccupancyTeacher)
	err := r.db.NewSelect().Model(roomOccupancyTeacher).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return roomOccupancyTeacher, nil
}

// FindByRoomOccupancy retrieves all room occupancy teachers for a room occupancy
func (r *RoomOccupancyTeacherRepository) FindByRoomOccupancy(ctx context.Context, roomOccupancyID int64) ([]*facilities.RoomOccupancyTeacher, error) {
	var roomOccupancyTeachers []*facilities.RoomOccupancyTeacher
	err := r.db.NewSelect().
		Model(&roomOccupancyTeachers).
		Where("room_occupancy_id = ?", roomOccupancyID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_room_occupancy", Err: err}
	}
	return roomOccupancyTeachers, nil
}

// FindByTeacher retrieves all room occupancy teachers for a teacher
func (r *RoomOccupancyTeacherRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*facilities.RoomOccupancyTeacher, error) {
	var roomOccupancyTeachers []*facilities.RoomOccupancyTeacher
	err := r.db.NewSelect().
		Model(&roomOccupancyTeachers).
		Where("teacher_id = ?", teacherID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_teacher", Err: err}
	}
	return roomOccupancyTeachers, nil
}

// DeleteByRoomOccupancy deletes all room occupancy teachers for a room occupancy
func (r *RoomOccupancyTeacherRepository) DeleteByRoomOccupancy(ctx context.Context, roomOccupancyID int64) error {
	_, err := r.db.NewDelete().
		Model((*facilities.RoomOccupancyTeacher)(nil)).
		Where("room_occupancy_id = ?", roomOccupancyID).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "delete_by_room_occupancy", Err: err}
	}
	return nil
}

// Update updates an existing room occupancy teacher
func (r *RoomOccupancyTeacherRepository) Update(ctx context.Context, roomOccupancyTeacher *facilities.RoomOccupancyTeacher) error {
	if err := roomOccupancyTeacher.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(roomOccupancyTeacher).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a room occupancy teacher
func (r *RoomOccupancyTeacherRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*facilities.RoomOccupancyTeacher)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves room occupancy teachers matching the filters
func (r *RoomOccupancyTeacherRepository) List(ctx context.Context, filters map[string]interface{}) ([]*facilities.RoomOccupancyTeacher, error) {
	var roomOccupancyTeachers []*facilities.RoomOccupancyTeacher
	query := r.db.NewSelect().Model(&roomOccupancyTeachers)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return roomOccupancyTeachers, nil
}
