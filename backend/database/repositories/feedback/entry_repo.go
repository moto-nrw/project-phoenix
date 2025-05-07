package feedback

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/feedback"
	"github.com/uptrace/bun"
)

// EntryRepository implements feedback.EntryRepository
type EntryRepository struct {
	db *bun.DB
}

// NewEntryRepository creates a new entry repository
func NewEntryRepository(db *bun.DB) feedback.EntryRepository {
	return &EntryRepository{db: db}
}

// Create inserts a new entry into the database
func (r *EntryRepository) Create(ctx context.Context, entry *feedback.Entry) error {
	if err := entry.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(entry).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves an entry by its ID
func (r *EntryRepository) FindByID(ctx context.Context, id interface{}) (*feedback.Entry, error) {
	entry := new(feedback.Entry)
	err := r.db.NewSelect().Model(entry).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return entry, nil
}

// FindByStudent retrieves all entries for a student
func (r *EntryRepository) FindByStudent(ctx context.Context, studentID int64) ([]*feedback.Entry, error) {
	var entries []*feedback.Entry
	err := r.db.NewSelect().
		Model(&entries).
		Where("student_id = ?", studentID).
		Order("day DESC, time DESC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_student", Err: err}
	}
	return entries, nil
}

// FindByDay retrieves all entries for a specific day
func (r *EntryRepository) FindByDay(ctx context.Context, day time.Time) ([]*feedback.Entry, error) {
	// Convert day to date-only format for comparison
	dayOnly := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())

	var entries []*feedback.Entry
	err := r.db.NewSelect().
		Model(&entries).
		Where("day = ?", dayOnly).
		Order("time ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_day", Err: err}
	}
	return entries, nil
}

// FindByDateRange retrieves all entries within a date range
func (r *EntryRepository) FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*feedback.Entry, error) {
	// Convert to date-only format for comparison
	startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
	endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, endDate.Location())

	var entries []*feedback.Entry
	err := r.db.NewSelect().
		Model(&entries).
		Where("day BETWEEN ? AND ?", startDate, endDate).
		Order("day DESC, time DESC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_date_range", Err: err}
	}
	return entries, nil
}

// FindMensaFeedback retrieves all mensa feedback entries
func (r *EntryRepository) FindMensaFeedback(ctx context.Context) ([]*feedback.Entry, error) {
	var entries []*feedback.Entry
	err := r.db.NewSelect().
		Model(&entries).
		Where("is_mensa_feedback = ?", true).
		Order("day DESC, time DESC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_mensa_feedback", Err: err}
	}
	return entries, nil
}

// FindRegularFeedback retrieves all non-mensa feedback entries
func (r *EntryRepository) FindRegularFeedback(ctx context.Context) ([]*feedback.Entry, error) {
	var entries []*feedback.Entry
	err := r.db.NewSelect().
		Model(&entries).
		Where("is_mensa_feedback = ?", false).
		Order("day DESC, time DESC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_regular_feedback", Err: err}
	}
	return entries, nil
}

// FindWithStudent retrieves an entry with its associated student data
func (r *EntryRepository) FindWithStudent(ctx context.Context, id int64) (*feedback.Entry, error) {
	entry := new(feedback.Entry)
	err := r.db.NewSelect().
		Model(entry).
		Relation("Student").
		Where("entries.id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_with_student", Err: err}
	}
	return entry, nil
}

// Update updates an existing entry
func (r *EntryRepository) Update(ctx context.Context, entry *feedback.Entry) error {
	if err := entry.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(entry).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes an entry
func (r *EntryRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*feedback.Entry)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves entries matching the filters
func (r *EntryRepository) List(ctx context.Context, filters map[string]interface{}) ([]*feedback.Entry, error) {
	var entries []*feedback.Entry
	query := r.db.NewSelect().Model(&entries)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return entries, nil
}
