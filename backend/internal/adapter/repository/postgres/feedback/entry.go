package feedback

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/feedback"
	feedbackPort "github.com/moto-nrw/project-phoenix/internal/core/port/feedback"
	"github.com/uptrace/bun"
)

// Table and query constants (S1192 - avoid duplicate string literals)
const (
	tableFeedbackEntries      = "feedback.entries"
	tableFeedbackEntriesAlias = `feedback.entries AS "entry"`
	orderByDayTimeDesc        = "day DESC, time DESC"
	whereIsMensaFeedback      = "is_mensa_feedback = ?"
)

// EntryRepository implements feedback.EntryRepository interface
type EntryRepository struct {
	*base.Repository[*feedback.Entry]
	db *bun.DB
}

// NewEntryRepository creates a new EntryRepository
func NewEntryRepository(db *bun.DB) feedbackPort.EntryRepository {
	return &EntryRepository{
		Repository: base.NewRepository[*feedback.Entry](db, tableFeedbackEntries, "Entry"),
		db:         db,
	}
}

// FindByID retrieves a feedback entry by its ID
// Returns (nil, nil) if no entry is found
func (r *EntryRepository) FindByID(ctx context.Context, id interface{}) (*feedback.Entry, error) {
	entry := new(feedback.Entry)
	err := r.db.NewSelect().
		Model(entry).
		ModelTableExpr(tableFeedbackEntriesAlias).
		Where(`"entry".id = ?`, id).
		Scan(ctx)

	if err != nil {
		// Return (nil, nil) for not found to allow service layer to handle it
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find by id",
			Err: err,
		}
	}

	return entry, nil
}

// FindByStudentID retrieves feedback entries by student ID
func (r *EntryRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*feedback.Entry, error) {
	var entries []*feedback.Entry
	err := r.db.NewSelect().
		Model(&entries).
		ModelTableExpr(tableFeedbackEntriesAlias).
		Where("student_id = ?", studentID).
		Order(orderByDayTimeDesc).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ID",
			Err: err,
		}
	}

	return entries, nil
}

// FindByDay retrieves feedback entries for a specific day
func (r *EntryRepository) FindByDay(ctx context.Context, day time.Time) ([]*feedback.Entry, error) {
	var entries []*feedback.Entry
	err := r.db.NewSelect().
		Model(&entries).
		ModelTableExpr(tableFeedbackEntriesAlias).
		Where("day = ?", day).
		Order("time DESC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by day",
			Err: err,
		}
	}

	return entries, nil
}

// FindByDateRange retrieves feedback entries within a date range
func (r *EntryRepository) FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*feedback.Entry, error) {
	var entries []*feedback.Entry
	err := r.db.NewSelect().
		Model(&entries).
		ModelTableExpr(tableFeedbackEntriesAlias).
		Where("day >= ? AND day <= ?", startDate, endDate).
		Order(orderByDayTimeDesc).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by date range",
			Err: err,
		}
	}

	return entries, nil
}

// FindMensaFeedback retrieves feedback entries related to the cafeteria
func (r *EntryRepository) FindMensaFeedback(ctx context.Context, isMensaFeedback bool) ([]*feedback.Entry, error) {
	var entries []*feedback.Entry
	err := r.db.NewSelect().
		Model(&entries).
		ModelTableExpr(tableFeedbackEntriesAlias).
		Where(whereIsMensaFeedback, isMensaFeedback).
		Order(orderByDayTimeDesc).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find mensa feedback",
			Err: err,
		}
	}

	return entries, nil
}

// FindByStudentAndDateRange retrieves feedback entries for a student within a date range
func (r *EntryRepository) FindByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate time.Time) ([]*feedback.Entry, error) {
	var entries []*feedback.Entry
	err := r.db.NewSelect().
		Model(&entries).
		ModelTableExpr(tableFeedbackEntriesAlias).
		Where("student_id = ? AND day >= ? AND day <= ?", studentID, startDate, endDate).
		Order(orderByDayTimeDesc).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student and date range",
			Err: err,
		}
	}

	return entries, nil
}

// CountByDay counts feedback entries for a specific day
func (r *EntryRepository) CountByDay(ctx context.Context, day time.Time) (int, error) {
	count, err := r.db.NewSelect().
		Model((*feedback.Entry)(nil)).
		ModelTableExpr(tableFeedbackEntriesAlias).
		Where("day = ?", day).
		Count(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count by day",
			Err: err,
		}
	}

	return count, nil
}

// CountByStudentID counts feedback entries for a specific student
func (r *EntryRepository) CountByStudentID(ctx context.Context, studentID int64) (int, error) {
	count, err := r.db.NewSelect().
		Model((*feedback.Entry)(nil)).
		ModelTableExpr(tableFeedbackEntriesAlias).
		Where("student_id = ?", studentID).
		Count(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count by student ID",
			Err: err,
		}
	}

	return count, nil
}

// CountMensaFeedback counts feedback entries related to the cafeteria
func (r *EntryRepository) CountMensaFeedback(ctx context.Context, isMensaFeedback bool) (int, error) {
	count, err := r.db.NewSelect().
		Model((*feedback.Entry)(nil)).
		ModelTableExpr(tableFeedbackEntriesAlias).
		Where(whereIsMensaFeedback, isMensaFeedback).
		Count(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count mensa feedback",
			Err: err,
		}
	}

	return count, nil
}

// Create overrides the base Create method to handle validation
func (r *EntryRepository) Create(ctx context.Context, entry *feedback.Entry) error {
	if entry == nil {
		return fmt.Errorf("entry cannot be nil")
	}

	// Validate entry
	if err := entry.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, entry)
}

// Update overrides the base Update method to handle validation
func (r *EntryRepository) Update(ctx context.Context, entry *feedback.Entry) error {
	if entry == nil {
		return fmt.Errorf("entry cannot be nil")
	}

	// Validate entry
	if err := entry.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, entry)
}

// List retrieves entries matching the provided filters
func (r *EntryRepository) List(ctx context.Context, filters map[string]interface{}) ([]*feedback.Entry, error) {
	var entries []*feedback.Entry
	query := r.db.NewSelect().Model(&entries).ModelTableExpr(tableFeedbackEntriesAlias)

	query = applyFeedbackFilters(query, filters)
	query = query.Order("day DESC, time DESC")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list", Err: err}
	}

	return entries, nil
}

// applyFeedbackFilters applies all filters to the query
func applyFeedbackFilters(query *bun.SelectQuery, filters map[string]interface{}) *bun.SelectQuery {
	for field, value := range filters {
		if value == nil {
			continue
		}
		query = applyFeedbackFilter(query, field, value)
	}
	return query
}

// applyFeedbackFilter applies a single filter to the query
func applyFeedbackFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	switch field {
	case "is_mensa_feedback":
		return query.Where("is_mensa_feedback = ?", value)
	case "day_from":
		if dateValue, ok := value.(time.Time); ok {
			return query.Where("day >= ?", dateValue)
		}
	case "day_to":
		if dateValue, ok := value.(time.Time); ok {
			return query.Where("day <= ?", dateValue)
		}
	case "value_like":
		if strValue, ok := value.(string); ok {
			return query.Where("value ILIKE ?", "%"+strValue+"%")
		}
	default:
		return query.Where("? = ?", bun.Ident(field), value)
	}
	return query
}
