package schedule

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// DateframeRepository implements schedule.DateframeRepository
type DateframeRepository struct {
	db *bun.DB
}

// NewDateframeRepository creates a new dateframe repository
func NewDateframeRepository(db *bun.DB) schedule.DateframeRepository {
	return &DateframeRepository{db: db}
}

// Create inserts a new dateframe into the database
func (r *DateframeRepository) Create(ctx context.Context, dateframe *schedule.Dateframe) error {
	if err := dateframe.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(dateframe).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a dateframe by its ID
func (r *DateframeRepository) FindByID(ctx context.Context, id interface{}) (*schedule.Dateframe, error) {
	dateframe := new(schedule.Dateframe)
	err := r.db.NewSelect().Model(dateframe).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return dateframe, nil
}

// FindByName retrieves dateframes by name (partial match)
func (r *DateframeRepository) FindByName(ctx context.Context, name string) ([]*schedule.Dateframe, error) {
	var dateframes []*schedule.Dateframe
	err := r.db.NewSelect().
		Model(&dateframes).
		Where("name ILIKE ?", "%"+name+"%").
		Order("start_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_name", Err: err}
	}
	return dateframes, nil
}

// FindByDateRange retrieves all dateframes within a date range
func (r *DateframeRepository) FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*schedule.Dateframe, error) {
	var dateframes []*schedule.Dateframe
	err := r.db.NewSelect().
		Model(&dateframes).
		Where("(start_date BETWEEN ? AND ?) OR (end_date BETWEEN ? AND ?) OR (start_date <= ? AND end_date >= ?)",
			startDate, endDate, startDate, endDate, startDate, endDate).
		Order("start_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_date_range", Err: err}
	}
	return dateframes, nil
}

// FindActive retrieves all currently active dateframes
func (r *DateframeRepository) FindActive(ctx context.Context) ([]*schedule.Dateframe, error) {
	now := time.Now()
	var dateframes []*schedule.Dateframe
	err := r.db.NewSelect().
		Model(&dateframes).
		Where("start_date <= ?", now).
		Where("end_date >= ?", now).
		Order("start_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_active", Err: err}
	}
	return dateframes, nil
}

// FindUpcoming retrieves all upcoming dateframes
func (r *DateframeRepository) FindUpcoming(ctx context.Context) ([]*schedule.Dateframe, error) {
	now := time.Now()
	var dateframes []*schedule.Dateframe
	err := r.db.NewSelect().
		Model(&dateframes).
		Where("start_date > ?", now).
		Order("start_date ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_upcoming", Err: err}
	}
	return dateframes, nil
}

// FindPast retrieves all past dateframes
func (r *DateframeRepository) FindPast(ctx context.Context) ([]*schedule.Dateframe, error) {
	now := time.Now()
	var dateframes []*schedule.Dateframe
	err := r.db.NewSelect().
		Model(&dateframes).
		Where("end_date < ?", now).
		Order("end_date DESC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_past", Err: err}
	}
	return dateframes, nil
}

// Update updates an existing dateframe
func (r *DateframeRepository) Update(ctx context.Context, dateframe *schedule.Dateframe) error {
	if err := dateframe.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(dateframe).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a dateframe
func (r *DateframeRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*schedule.Dateframe)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves dateframes matching the filters
func (r *DateframeRepository) List(ctx context.Context, filters map[string]interface{}) ([]*schedule.Dateframe, error) {
	var dateframes []*schedule.Dateframe
	query := r.db.NewSelect().Model(&dateframes)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return dateframes, nil
}
