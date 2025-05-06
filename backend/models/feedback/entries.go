package feedback

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Entry represents a feedback entry in the system
type Entry struct {
	base.Model
	Value           string    `bun:"value,notnull" json:"value"`
	Day             time.Time `bun:"day,notnull" json:"day"`
	Time            time.Time `bun:"time,notnull" json:"time"`
	StudentID       int64     `bun:"student_id,notnull" json:"student_id"`
	IsMensaFeedback bool      `bun:"is_mensa_feedback,notnull,default:false" json:"is_mensa_feedback"`

	// Relations
	Student *users.Student `bun:"rel:belongs-to,join:student_id=id" json:"student,omitempty"`
}

// TableName returns the table name for the Entry model
func (e *Entry) TableName() string {
	return "feedback.entries"
}

// GetID returns the entry ID
func (e *Entry) GetID() interface{} {
	return e.ID
}

// GetCreatedAt returns the creation timestamp
func (e *Entry) GetCreatedAt() time.Time {
	return e.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (e *Entry) GetUpdatedAt() time.Time {
	return e.UpdatedAt
}

// Validate validates the entry fields
func (e *Entry) Validate() error {
	if e.Value == "" {
		return errors.New("feedback value is required")
	}

	if e.Day.IsZero() {
		return errors.New("day is required")
	}

	if e.Time.IsZero() {
		return errors.New("time is required")
	}

	if e.StudentID <= 0 {
		return errors.New("student ID is required")
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (e *Entry) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := e.Model.BeforeAppend(); err != nil {
		return err
	}

	// Set defaults for day and time if not provided
	if e.Day.IsZero() {
		now := time.Now()
		e.Day = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	if e.Time.IsZero() {
		now := time.Now()
		e.Time = time.Date(0, 1, 1, now.Hour(), now.Minute(), now.Second(), 0, now.Location())
	}

	return nil
}

// EntryRepository defines operations for working with feedback entries
type EntryRepository interface {
	base.Repository[*Entry]
	FindByStudent(ctx context.Context, studentID int64) ([]*Entry, error)
	FindByDay(ctx context.Context, day time.Time) ([]*Entry, error)
	FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*Entry, error)
	FindMensaFeedback(ctx context.Context) ([]*Entry, error)
	FindRegularFeedback(ctx context.Context) ([]*Entry, error)
	FindWithStudent(ctx context.Context, id int64) (*Entry, error)
}

// DefaultEntryRepository is the default implementation of EntryRepository
type DefaultEntryRepository struct {
	db *bun.DB
}

// NewEntryRepository creates a new entry repository
func NewEntryRepository(db *bun.DB) EntryRepository {
	return &DefaultEntryRepository{db: db}
}

// Create inserts a new entry into the database
func (r *DefaultEntryRepository) Create(ctx context.Context, entry *Entry) error {
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
func (r *DefaultEntryRepository) FindByID(ctx context.Context, id interface{}) (*Entry, error) {
	entry := new(Entry)
	err := r.db.NewSelect().Model(entry).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return entry, nil
}

// FindByStudent retrieves all entries for a student
func (r *DefaultEntryRepository) FindByStudent(ctx context.Context, studentID int64) ([]*Entry, error) {
	var entries []*Entry
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
func (r *DefaultEntryRepository) FindByDay(ctx context.Context, day time.Time) ([]*Entry, error) {
	// Convert day to date-only format for comparison
	dayOnly := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())

	var entries []*Entry
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
func (r *DefaultEntryRepository) FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*Entry, error) {
	// Convert to date-only format for comparison
	startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
	endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, endDate.Location())

	var entries []*Entry
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
func (r *DefaultEntryRepository) FindMensaFeedback(ctx context.Context) ([]*Entry, error) {
	var entries []*Entry
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
func (r *DefaultEntryRepository) FindRegularFeedback(ctx context.Context) ([]*Entry, error) {
	var entries []*Entry
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
func (r *DefaultEntryRepository) FindWithStudent(ctx context.Context, id int64) (*Entry, error) {
	entry := new(Entry)
	err := r.db.NewSelect().
		Model(entry).
		Relation("Student").
		Where("entry.id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_with_student", Err: err}
	}
	return entry, nil
}

// Update updates an existing entry
func (r *DefaultEntryRepository) Update(ctx context.Context, entry *Entry) error {
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
func (r *DefaultEntryRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Entry)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves entries matching the filters
func (r *DefaultEntryRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Entry, error) {
	var entries []*Entry
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
