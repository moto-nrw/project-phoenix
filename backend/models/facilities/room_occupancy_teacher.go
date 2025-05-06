package facilities

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// RoomOccupancyTeacher represents a teacher assigned to a room occupancy
type RoomOccupancyTeacher struct {
	base.Model
	RoomOccupancyID int64 `bun:"room_occupancy_id,notnull" json:"room_occupancy_id"`
	TeacherID       int64 `bun:"teacher_id,notnull" json:"teacher_id"`

	// Relations
	RoomOccupancy *RoomOccupancy `bun:"rel:belongs-to,join:room_occupancy_id=id" json:"room_occupancy,omitempty"`
	Teacher       *users.Teacher `bun:"rel:belongs-to,join:teacher_id=id" json:"teacher,omitempty"`
}

// TableName returns the table name for the RoomOccupancyTeacher model
func (r *RoomOccupancyTeacher) TableName() string {
	return "facilities.room_occupancy_teachers"
}

// GetID returns the room occupancy teacher ID
func (r *RoomOccupancyTeacher) GetID() interface{} {
	return r.ID
}

// GetCreatedAt returns the creation timestamp
func (r *RoomOccupancyTeacher) GetCreatedAt() time.Time {
	return r.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (r *RoomOccupancyTeacher) GetUpdatedAt() time.Time {
	return r.CreatedAt // This model only has created_at, no updated_at
}

// Validate validates the room occupancy teacher fields
func (r *RoomOccupancyTeacher) Validate() error {
	if r.RoomOccupancyID <= 0 {
		return errors.New("room occupancy ID is required")
	}

	if r.TeacherID <= 0 {
		return errors.New("teacher ID is required")
	}

	return nil
}

// RoomOccupancyTeacherRepository defines operations for working with room occupancy teachers
type RoomOccupancyTeacherRepository interface {
	base.Repository[*RoomOccupancyTeacher]
	FindByRoomOccupancy(ctx context.Context, roomOccupancyID int64) ([]*RoomOccupancyTeacher, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*RoomOccupancyTeacher, error)
	DeleteByRoomOccupancy(ctx context.Context, roomOccupancyID int64) error
}

// DefaultRoomOccupancyTeacherRepository is the default implementation of RoomOccupancyTeacherRepository
type DefaultRoomOccupancyTeacherRepository struct {
	db *bun.DB
}

// NewRoomOccupancyTeacherRepository creates a new room occupancy teacher repository
func NewRoomOccupancyTeacherRepository(db *bun.DB) RoomOccupancyTeacherRepository {
	return &DefaultRoomOccupancyTeacherRepository{db: db}
}

// Create inserts a new room occupancy teacher into the database
func (r *DefaultRoomOccupancyTeacherRepository) Create(ctx context.Context, roomOccupancyTeacher *RoomOccupancyTeacher) error {
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
func (r *DefaultRoomOccupancyTeacherRepository) FindByID(ctx context.Context, id interface{}) (*RoomOccupancyTeacher, error) {
	roomOccupancyTeacher := new(RoomOccupancyTeacher)
	err := r.db.NewSelect().Model(roomOccupancyTeacher).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return roomOccupancyTeacher, nil
}

// FindByRoomOccupancy retrieves all room occupancy teachers for a room occupancy
func (r *DefaultRoomOccupancyTeacherRepository) FindByRoomOccupancy(ctx context.Context, roomOccupancyID int64) ([]*RoomOccupancyTeacher, error) {
	var roomOccupancyTeachers []*RoomOccupancyTeacher
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
func (r *DefaultRoomOccupancyTeacherRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*RoomOccupancyTeacher, error) {
	var roomOccupancyTeachers []*RoomOccupancyTeacher
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
func (r *DefaultRoomOccupancyTeacherRepository) DeleteByRoomOccupancy(ctx context.Context, roomOccupancyID int64) error {
	_, err := r.db.NewDelete().
		Model((*RoomOccupancyTeacher)(nil)).
		Where("room_occupancy_id = ?", roomOccupancyID).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "delete_by_room_occupancy", Err: err}
	}
	return nil
}

// Update updates an existing room occupancy teacher
func (r *DefaultRoomOccupancyTeacherRepository) Update(ctx context.Context, roomOccupancyTeacher *RoomOccupancyTeacher) error {
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
func (r *DefaultRoomOccupancyTeacherRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*RoomOccupancyTeacher)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves room occupancy teachers matching the filters
func (r *DefaultRoomOccupancyTeacherRepository) List(ctx context.Context, filters map[string]interface{}) ([]*RoomOccupancyTeacher, error) {
	var roomOccupancyTeachers []*RoomOccupancyTeacher
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
