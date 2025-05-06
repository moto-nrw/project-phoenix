package facilities

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Visit represents a person's visit to a room
type Visit struct {
	base.Model
	PersonID        int64      `bun:"person_id,notnull" json:"person_id"`
	RoomOccupancyID int64      `bun:"room_occupancy_id,notnull" json:"room_occupancy_id"`
	EntryTime       time.Time  `bun:"entry_time,notnull" json:"entry_time"`
	ExitTime        *time.Time `bun:"exit_time" json:"exit_time,omitempty"`

	// Relations
	Person        *users.Person  `bun:"rel:belongs-to,join:person_id=id" json:"person,omitempty"`
	RoomOccupancy *RoomOccupancy `bun:"rel:belongs-to,join:room_occupancy_id=id" json:"room_occupancy,omitempty"`
}

// TableName returns the table name for the Visit model
func (v *Visit) TableName() string {
	return "facilities.visits"
}

// GetID returns the visit ID
func (v *Visit) GetID() interface{} {
	return v.ID
}

// GetCreatedAt returns the creation timestamp
func (v *Visit) GetCreatedAt() time.Time {
	return v.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (v *Visit) GetUpdatedAt() time.Time {
	return v.UpdatedAt
}

// Validate validates the visit fields
func (v *Visit) Validate() error {
	if v.PersonID <= 0 {
		return errors.New("person ID is required")
	}

	if v.RoomOccupancyID <= 0 {
		return errors.New("room occupancy ID is required")
	}

	if v.EntryTime.IsZero() {
		return errors.New("entry time is required")
	}

	if v.ExitTime != nil && !v.ExitTime.IsZero() && v.ExitTime.Before(v.EntryTime) {
		return errors.New("exit time must be after entry time")
	}

	return nil
}

// IsActive checks if the visit is active (no exit time)
func (v *Visit) IsActive() bool {
	return v.ExitTime == nil || v.ExitTime.IsZero()
}

// Duration returns the duration of the visit
func (v *Visit) Duration() time.Duration {
	if v.IsActive() {
		return time.Since(v.EntryTime)
	}
	return v.ExitTime.Sub(v.EntryTime)
}

// VisitRepository defines operations for working with visits
type VisitRepository interface {
	base.Repository[*Visit]
	FindByPerson(ctx context.Context, personID int64) ([]*Visit, error)
	FindByRoomOccupancy(ctx context.Context, roomOccupancyID int64) ([]*Visit, error)
	FindActive(ctx context.Context) ([]*Visit, error)
	FindActiveByPerson(ctx context.Context, personID int64) ([]*Visit, error)
	FindActiveByRoomOccupancy(ctx context.Context, roomOccupancyID int64) ([]*Visit, error)
	RecordExit(ctx context.Context, id int64, exitTime time.Time) error
	GetStats(ctx context.Context, roomID int64, startDate, endDate time.Time) (map[string]interface{}, error)
}

// DefaultVisitRepository is the default implementation of VisitRepository
type DefaultVisitRepository struct {
	db *bun.DB
}

// NewVisitRepository creates a new visit repository
func NewVisitRepository(db *bun.DB) VisitRepository {
	return &DefaultVisitRepository{db: db}
}

// Create inserts a new visit into the database
func (r *DefaultVisitRepository) Create(ctx context.Context, visit *Visit) error {
	if err := visit.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(visit).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a visit by its ID
func (r *DefaultVisitRepository) FindByID(ctx context.Context, id interface{}) (*Visit, error) {
	visit := new(Visit)
	err := r.db.NewSelect().Model(visit).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return visit, nil
}

// FindByPerson retrieves all visits for a person
func (r *DefaultVisitRepository) FindByPerson(ctx context.Context, personID int64) ([]*Visit, error) {
	var visits []*Visit
	err := r.db.NewSelect().
		Model(&visits).
		Where("person_id = ?", personID).
		Order("entry_time DESC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_person", Err: err}
	}
	return visits, nil
}

// FindByRoomOccupancy retrieves all visits for a room occupancy
func (r *DefaultVisitRepository) FindByRoomOccupancy(ctx context.Context, roomOccupancyID int64) ([]*Visit, error) {
	var visits []*Visit
	err := r.db.NewSelect().
		Model(&visits).
		Where("room_occupancy_id = ?", roomOccupancyID).
		Order("entry_time DESC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_room_occupancy", Err: err}
	}
	return visits, nil
}

// FindActive retrieves all active visits (with no exit time)
func (r *DefaultVisitRepository) FindActive(ctx context.Context) ([]*Visit, error) {
	var visits []*Visit
	err := r.db.NewSelect().
		Model(&visits).
		Where("exit_time IS NULL").
		Order("entry_time DESC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active", Err: err}
	}
	return visits, nil
}

// FindActiveByPerson retrieves all active visits for a person
func (r *DefaultVisitRepository) FindActiveByPerson(ctx context.Context, personID int64) ([]*Visit, error) {
	var visits []*Visit
	err := r.db.NewSelect().
		Model(&visits).
		Where("person_id = ?", personID).
		Where("exit_time IS NULL").
		Order("entry_time DESC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active_by_person", Err: err}
	}
	return visits, nil
}

// FindActiveByRoomOccupancy retrieves all active visits for a room occupancy
func (r *DefaultVisitRepository) FindActiveByRoomOccupancy(ctx context.Context, roomOccupancyID int64) ([]*Visit, error) {
	var visits []*Visit
	err := r.db.NewSelect().
		Model(&visits).
		Where("room_occupancy_id = ?", roomOccupancyID).
		Where("exit_time IS NULL").
		Order("entry_time DESC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active_by_room_occupancy", Err: err}
	}
	return visits, nil
}

// RecordExit records an exit time for a visit
func (r *DefaultVisitRepository) RecordExit(ctx context.Context, id int64, exitTime time.Time) error {
	// First check if the visit exists and validate the exit time
	visit, err := r.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if !exitTime.IsZero() && exitTime.Before(visit.EntryTime) {
		return errors.New("exit time must be after entry time")
	}

	// Update the exit time
	_, err = r.db.NewUpdate().
		Model((*Visit)(nil)).
		Set("exit_time = ?", exitTime).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "record_exit", Err: err}
	}
	return nil
}

// GetStats retrieves visit statistics for a room
func (r *DefaultVisitRepository) GetStats(ctx context.Context, roomID int64, startDate, endDate time.Time) (map[string]interface{}, error) {
	var totalVisits int
	var avgDuration float64
	var peakOccupancy int

	// Query for total visits
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM facilities.visits v
		JOIN facilities.room_occupancy ro ON v.room_occupancy_id = ro.id
		WHERE ro.room_id = ? AND v.entry_time BETWEEN ? AND ?
	`, roomID, startDate, endDate).Scan(&totalVisits)

	if err != nil {
		return nil, &base.DatabaseError{Op: "get_stats_total", Err: err}
	}

	// Query for average duration (only for completed visits)
	err = r.db.QueryRowContext(ctx, `
		SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (v.exit_time - v.entry_time))), 0)
		FROM facilities.visits v
		JOIN facilities.room_occupancy ro ON v.room_occupancy_id = ro.id
		WHERE ro.room_id = ? AND v.entry_time BETWEEN ? AND ? AND v.exit_time IS NOT NULL
	`, roomID, startDate, endDate).Scan(&avgDuration)

	if err != nil {
		return nil, &base.DatabaseError{Op: "get_stats_avg_duration", Err: err}
	}

	// Query for peak occupancy
	err = r.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(concurrent_visits), 0)
		FROM (
			SELECT COUNT(*) as concurrent_visits
			FROM facilities.visits v
			JOIN facilities.room_occupancy ro ON v.room_occupancy_id = ro.id
			WHERE ro.room_id = ? AND 
				((v.entry_time BETWEEN ? AND ?) OR
				(v.exit_time IS NULL) OR
				(v.exit_time BETWEEN ? AND ?))
			GROUP BY DATE_TRUNC('hour', v.entry_time)
		) as hourly_visits
	`, roomID, startDate, endDate, startDate, endDate).Scan(&peakOccupancy)

	if err != nil {
		return nil, &base.DatabaseError{Op: "get_stats_peak", Err: err}
	}

	return map[string]interface{}{
		"total_visits":     totalVisits,
		"avg_duration_sec": avgDuration,
		"peak_occupancy":   peakOccupancy,
	}, nil
}

// Update updates an existing visit
func (r *DefaultVisitRepository) Update(ctx context.Context, visit *Visit) error {
	if err := visit.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(visit).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a visit
func (r *DefaultVisitRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Visit)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves visits matching the filters
func (r *DefaultVisitRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Visit, error) {
	var visits []*Visit
	query := r.db.NewSelect().Model(&visits)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return visits, nil
}
