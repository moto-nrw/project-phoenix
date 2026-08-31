package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/feedback/internal/domain"
	"github.com/uptrace/bun"
)

type Database func(context.Context) (bun.IDB, int64, error)

type Store struct{ database Database }

type row struct {
	bun.BaseModel   `bun:"table:feedback.entries,alias:entry"`
	ID              int64         `bun:"id,pk,autoincrement"`
	TenantID        int64         `bun:"tenant_id,notnull"`
	Value           string        `bun:"value,notnull"`
	Day             timezone.Date `bun:"day,notnull,type:date"`
	Time            time.Time     `bun:"time,notnull,type:time"`
	StudentID       int64         `bun:"student_id,notnull"`
	IsMensaFeedback bool          `bun:"is_mensa_feedback,notnull"`
	CreatedAt       time.Time     `bun:"created_at"`
	UpdatedAt       time.Time     `bun:"updated_at"`
}

func New(database Database) *Store {
	if database == nil {
		panic("feedback postgres: database runtime is required")
	}
	return &Store{database: database}
}

func (s *Store) Create(ctx context.Context, entry domain.Entry) (domain.Entry, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Entry{}, domain.OperationStats{}, err
	}
	value := row{TenantID: tenantID, Value: entry.Value, Day: entry.Day, Time: entry.Time, StudentID: entry.StudentID, IsMensaFeedback: entry.IsMensaFeedback}
	query := db.NewInsert().Model(&value).ModelTableExpr(`feedback.entries`).Returning("*")
	started := time.Now()
	err = query.Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.Entry{}, stats, fmt.Errorf("feedback postgres: insert entry: %w", err)
	}
	stats.Rows = 1
	return toDomain(value), stats, nil
}

func (s *Store) Get(ctx context.Context, id int64) (domain.Entry, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Entry{}, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	var value row
	err = db.NewSelect().Model(&value).
		ModelTableExpr(`feedback.entries AS "entry"`).
		Where(`"entry".tenant_id = ?`, tenantID).
		Where(`"entry".id = ?`, id).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Entry{}, stats, domain.ErrEntryNotFound
	}
	if err != nil {
		return domain.Entry{}, stats, fmt.Errorf("feedback postgres: get entry: %w", err)
	}
	return toDomain(value), stats, nil
}

func (s *Store) Delete(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewDelete().Model((*row)(nil)).
		ModelTableExpr(`feedback.entries AS "entry"`).
		Where(`"entry".tenant_id = ?`, tenantID).
		Where(`"entry".id = ?`, id)
	started := time.Now()
	result, err := query.Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("feedback postgres: delete entry: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("feedback postgres: count deleted entries: %w", err)
	}
	if rows == 0 {
		return stats, domain.ErrEntryNotFound
	}
	stats.Rows = rows
	return stats, nil
}

func (s *Store) List(ctx context.Context, filter domain.Filter) ([]domain.Entry, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	var values []row
	query := db.NewSelect().Model(&values).
		ModelTableExpr(`feedback.entries AS "entry"`).
		Where(`"entry".tenant_id = ?`, tenantID)
	if filter.StudentID != nil {
		query = query.Where(`"entry".student_id = ?`, *filter.StudentID)
	}
	if filter.Day != nil {
		query = query.Where(`"entry".day = ?`, *filter.Day)
	}
	if filter.IsMensaFeedback != nil {
		query = query.Where(`"entry".is_mensa_feedback = ?`, *filter.IsMensaFeedback)
	}
	if filter.DayFrom != nil {
		query = query.Where(`"entry".day >= ?`, *filter.DayFrom)
	}
	if filter.DayTo != nil {
		query = query.Where(`"entry".day <= ?`, *filter.DayTo)
	}
	if filter.ValueLike != nil {
		query = query.Where(`"entry".value ILIKE ?`, "%"+*filter.ValueLike+"%")
	}
	err = query.OrderExpr(`"entry".day DESC`).OrderExpr(`"entry".time DESC`).Scan(ctx)
	if err != nil {
		return nil, stats, fmt.Errorf("feedback postgres: list entries: %w", err)
	}
	entries := make([]domain.Entry, 0, len(values))
	for _, value := range values {
		entries = append(entries, toDomain(value))
	}
	return entries, stats, nil
}

func (s *Store) DeleteOlderThan(ctx context.Context, cutoff domain.Date) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := db.NewDelete().Model((*row)(nil)).
		ModelTableExpr(`feedback.entries AS "entry"`).
		Where(`"entry".tenant_id = ?`, tenantID).
		Where(`"entry".day < ?`, cutoff)
	started := time.Now()
	result, err := query.Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("feedback postgres: delete expired entries: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("feedback postgres: count expired entries: %w", err)
	}
	stats.Rows = rows
	return stats, nil
}

func (s *Store) CountForStudent(ctx context.Context, studentID int64) (int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	count, err := db.NewSelect().Model((*row)(nil)).
		ModelTableExpr(`feedback.entries AS "entry"`).
		Where(`"entry".tenant_id = ?`, tenantID).
		Where(`"entry".student_id = ?`, studentID).
		Count(ctx)
	if err != nil {
		return 0, stats, fmt.Errorf("feedback postgres: count student entries: %w", err)
	}
	return count, stats, nil
}

func toDomain(value row) domain.Entry {
	return domain.Entry{
		ID: value.ID, Value: value.Value, Day: value.Day, Time: value.Time, StudentID: value.StudentID,
		IsMensaFeedback: value.IsMensaFeedback, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}
