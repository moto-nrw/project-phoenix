package schedule

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Dateframe represents a date range for planning
type Dateframe struct {
	base.Model
	StartDate   time.Time `bun:"start_date,notnull" json:"start_date"`
	EndDate     time.Time `bun:"end_date,notnull" json:"end_date"`
	Name        string    `bun:"name" json:"name,omitempty"`
	Description string    `bun:"description" json:"description,omitempty"`

	// Relations - we don't define backward relations to avoid circular imports
}

// TableName returns the table name for the Dateframe model
func (d *Dateframe) TableName() string {
	return "schedule.dateframes"
}

// GetID returns the dateframe ID
func (d *Dateframe) GetID() interface{} {
	return d.ID
}

// GetCreatedAt returns the creation timestamp
func (d *Dateframe) GetCreatedAt() time.Time {
	return d.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (d *Dateframe) GetUpdatedAt() time.Time {
	return d.UpdatedAt
}

// Validate validates the dateframe fields
func (d *Dateframe) Validate() error {
	if d.StartDate.IsZero() {
		return errors.New("start date is required")
	}

	if d.EndDate.IsZero() {
		return errors.New("end date is required")
	}

	if d.EndDate.Before(d.StartDate) {
		return errors.New("end date must be after start date")
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (d *Dateframe) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := d.Model.BeforeAppend(); err != nil {
		return err
	}

	// Trim whitespace
	d.Name = strings.TrimSpace(d.Name)
	d.Description = strings.TrimSpace(d.Description)

	return nil
}

// IsActive checks if the dateframe is currently active
func (d *Dateframe) IsActive() bool {
	now := time.Now()
	return !now.Before(d.StartDate) && !now.After(d.EndDate)
}

// DateframeRepository defines operations for working with dateframes
type DateframeRepository interface {
	base.Repository[*Dateframe]
	FindByName(ctx context.Context, name string) ([]*Dateframe, error)
	FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*Dateframe, error)
	FindActive(ctx context.Context) ([]*Dateframe, error)
	FindUpcoming(ctx context.Context) ([]*Dateframe, error)
	FindPast(ctx context.Context) ([]*Dateframe, error)
}

// DefaultDateframeRepository is the default implementation of DateframeRepository
type DefaultDateframeRepository struct {
	db *bun.DB
}

// NewDateframeRepository creates a new dateframe repository
func NewDateframeRepository(db *bun.DB) DateframeRepository {
	return &DefaultDateframeRepository{db: db}
}

// Create inserts a new dateframe into the database
func (r *DefaultDateframeRepository) Create(ctx context.Context, dateframe *Dateframe) error {
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
func (r *DefaultDateframeRepository) FindByID(ctx context.Context, id interface{}) (*Dateframe, error) {
	dateframe := new(Dateframe)
	err := r.db.NewSelect().Model(dateframe).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return dateframe, nil
}

// FindByName retrieves dateframes by name (partial match)
func (r *DefaultDateframeRepository) FindByName(ctx context.Context, name string) ([]*Dateframe, error) {
	var dateframes []*Dateframe
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
func (r *DefaultDateframeRepository) FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*Dateframe, error) {
	var dateframes []*Dateframe
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
func (r *DefaultDateframeRepository) FindActive(ctx context.Context) ([]*Dateframe, error) {
	now := time.Now()
	var dateframes []*Dateframe
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
func (r *DefaultDateframeRepository) FindUpcoming(ctx context.Context) ([]*Dateframe, error) {
	now := time.Now()
	var dateframes []*Dateframe
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
func (r *DefaultDateframeRepository) FindPast(ctx context.Context) ([]*Dateframe, error) {
	now := time.Now()
	var dateframes []*Dateframe
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
func (r *DefaultDateframeRepository) Update(ctx context.Context, dateframe *Dateframe) error {
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
func (r *DefaultDateframeRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Dateframe)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves dateframes matching the filters
func (r *DefaultDateframeRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Dateframe, error) {
	var dateframes []*Dateframe
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
