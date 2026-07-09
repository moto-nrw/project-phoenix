package feedback

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/feedback"
	"github.com/uptrace/bun"
)

// Table and query constants (S1192 - avoid duplicate string literals)
const (
	tableFeedbackEntries      = "feedback.entries"
	tableFeedbackEntriesAlias = `feedback.entries AS "entry"`
	whereIsMensaFeedback      = "is_mensa_feedback = ?"
	orderDayDesc              = "day DESC"
	orderTimeDesc             = "time DESC"
)

// EntryRepository implements feedback.EntryRepository interface
type EntryRepository struct {
	*base.Repository[*feedback.Entry]
	db *bun.DB
}

// NewEntryRepository creates a new EntryRepository
func NewEntryRepository(db *bun.DB) feedback.EntryRepository {
	repo := base.NewRepository[*feedback.Entry](db, tableFeedbackEntries, "Entry")
	repo.TenantScoped = true
	return &EntryRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByID retrieves a feedback entry by its ID
// Returns (nil, nil) if no entry is found
func (r *EntryRepository) FindByID(ctx context.Context, id interface{}) (*feedback.Entry, error) {
	return r.FindByIDOrNil(ctx, id)
}

// FindByStudentID retrieves feedback entries by student ID
func (r *EntryRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*feedback.Entry, error) {
	var entries []*feedback.Entry
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&entries).
		ModelTableExpr(tableFeedbackEntriesAlias).
		Where("student_id = ?", studentID)

	query = base.WithTenantFilter(ctx, query, "entry")

	err := query.
		Order(orderDayDesc).
		Order(orderTimeDesc).
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
func (r *EntryRepository) FindByDay(ctx context.Context, day timezone.Date) ([]*feedback.Entry, error) {
	var entries []*feedback.Entry
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&entries).
		ModelTableExpr(tableFeedbackEntriesAlias).
		Where("day = ?", day)

	query = base.WithTenantFilter(ctx, query, "entry")

	err := query.Order(orderTimeDesc).Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by day",
			Err: err,
		}
	}

	return entries, nil
}

// FindByDateRange retrieves feedback entries within a date range
func (r *EntryRepository) FindByDateRange(ctx context.Context, startDate, endDate timezone.Date) ([]*feedback.Entry, error) {
	var entries []*feedback.Entry
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&entries).
		ModelTableExpr(tableFeedbackEntriesAlias).
		Where("day >= ? AND day <= ?", startDate, endDate)

	query = base.WithTenantFilter(ctx, query, "entry")

	err := query.
		Order(orderDayDesc).
		Order(orderTimeDesc).
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&entries).
		ModelTableExpr(tableFeedbackEntriesAlias).
		Where(whereIsMensaFeedback, isMensaFeedback)

	query = base.WithTenantFilter(ctx, query, "entry")

	err := query.
		Order(orderDayDesc).
		Order(orderTimeDesc).
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
func (r *EntryRepository) FindByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*feedback.Entry, error) {
	var entries []*feedback.Entry
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&entries).
		ModelTableExpr(tableFeedbackEntriesAlias).
		Where("student_id = ? AND day >= ? AND day <= ?", studentID, startDate, endDate)

	query = base.WithTenantFilter(ctx, query, "entry")

	err := query.
		Order(orderDayDesc).
		Order(orderTimeDesc).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student and date range",
			Err: err,
		}
	}

	return entries, nil
}

// List retrieves entries matching the provided filters
func (r *EntryRepository) List(ctx context.Context, filters map[string]interface{}) ([]*feedback.Entry, error) {
	var entries []*feedback.Entry
	query := base.GetDB(ctx, r.db).NewSelect().Model(&entries).ModelTableExpr(tableFeedbackEntriesAlias)

	query = base.WithTenantFilter(ctx, query, "entry")

	query = applyFeedbackFilters(query, filters)
	query = query.Order(orderDayDesc).Order(orderTimeDesc)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list", Err: err}
	}

	return entries, nil
}

// DeleteOlderThan deletes feedback entries older than the specified number of days.
// Uses tenant scoping for GDPR-compliant per-tenant cleanup.
func (r *EntryRepository) DeleteOlderThan(ctx context.Context, days int) (int, error) {
	deleted, err := r.Repository.DeleteOlderThan(ctx, "day", timezone.TodayDate().AddDays(-days))
	return int(deleted), err
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
		if dateValue, ok := value.(timezone.Date); ok {
			return query.Where("day >= ?", dateValue)
		}
	case "day_to":
		if dateValue, ok := value.(timezone.Date); ok {
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
