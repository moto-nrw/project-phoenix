package facilities

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/uptrace/bun"
)

// VisitRepository implements facilities.VisitRepository
type VisitRepository struct {
	db *bun.DB
}

// NewVisitRepository creates a new visit repository
func NewVisitRepository(db *bun.DB) facilities.VisitRepository {
	return &VisitRepository{db: db}
}

// Create inserts a new visit into the database
func (r *VisitRepository) Create(ctx context.Context, visit *facilities.Visit) error {
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
func (r *VisitRepository) FindByID(ctx context.Context, id interface{}) (*facilities.Visit, error) {
	visit := new(facilities.Visit)
	err := r.db.NewSelect().Model(visit).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return visit, nil
}

// FindByPerson retrieves all visits for a person
func (r *VisitRepository) FindByPerson(ctx context.Context, personID int64) ([]*facilities.Visit, error) {
	var visits []*facilities.Visit
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
func (r *VisitRepository) FindByRoomOccupancy(ctx context.Context, roomOccupancyID int64) ([]*facilities.Visit, error) {
	var visits []*facilities.Visit
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
func (r *VisitRepository) FindActive(ctx context.Context) ([]*facilities.Visit, error) {
	var visits []*facilities.Visit
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
func (r *VisitRepository) FindActiveByPerson(ctx context.Context, personID int64) ([]*facilities.Visit, error) {
	var visits []*facilities.Visit
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
func (r *VisitRepository) FindActiveByRoomOccupancy(ctx context.Context, roomOccupancyID int64) ([]*facilities.Visit, error) {
	var visits []*facilities.Visit
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
func (r *VisitRepository) RecordExit(ctx context.Context, id int64, exitTime time.Time) error {
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
		Model((*facilities.Visit)(nil)).
		Set("exit_time = ?", exitTime).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "record_exit", Err: err}
	}
	return nil
}

// GetStats retrieves visit statistics for a room
func (r *VisitRepository) GetStats(ctx context.Context, roomID int64, startDate, endDate time.Time) (map[string]interface{}, error) {
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
func (r *VisitRepository) Update(ctx context.Context, visit *facilities.Visit) error {
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
func (r *VisitRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*facilities.Visit)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves visits matching the filters
func (r *VisitRepository) List(ctx context.Context, filters map[string]interface{}) ([]*facilities.Visit, error) {
	var visits []*facilities.Visit
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
