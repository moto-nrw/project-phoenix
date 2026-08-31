package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/uptrace/bun"
)

type Database func(context.Context) (bun.IDB, int64, error)

type Store struct{ database Database }

type row struct {
	bun.BaseModel `bun:"table:meal_plan_entries,alias:meal_plan_entry"`
	TenantID      int64         `bun:"tenant_id"`
	Date          timezone.Date `bun:"date,type:date"`
	Position      int           `bun:"position"`
	Dish          string        `bun:"dish"`
	Note          *string       `bun:"note"`
}

func New(database Database) *Store {
	if database == nil {
		panic("meal plan postgres: database runtime is required")
	}
	return &Store{database: database}
}

func (s *Store) FindWeek(ctx context.Context, start, end domain.Date) ([]domain.Entry, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	var rows []row
	err = db.NewSelect().Model(&rows).
		ModelTableExpr(`schedule.meal_plan_entries AS "meal_plan_entry"`).
		Where(`"meal_plan_entry".tenant_id = ?`, tenantID).
		Where(`"meal_plan_entry".date >= ?`, start).
		Where(`"meal_plan_entry".date <= ?`, end).
		OrderExpr(`"meal_plan_entry".date ASC`).
		OrderExpr(`"meal_plan_entry".position ASC`).
		Scan(ctx)
	if err != nil {
		return nil, stats, fmt.Errorf("meal plan postgres: find week: %w", err)
	}
	entries := make([]domain.Entry, 0, len(rows))
	for _, value := range rows {
		entries = append(entries, domain.Entry{Date: value.Date, Position: value.Position, Dish: value.Dish, Note: value.Note})
	}
	return entries, stats, nil
}

func (s *Store) ReplaceDay(ctx context.Context, date domain.Date, dishes []domain.Dish) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats, err := deleteDay(ctx, db, tenantID, date)
	if err != nil || len(dishes) == 0 {
		return stats, err
	}
	rows := make([]row, 0, len(dishes))
	for position, dish := range dishes {
		rows = append(rows, row{TenantID: tenantID, Date: date, Position: position, Dish: dish.Dish, Note: dish.Note})
	}
	query := db.NewInsert().Model(&rows).ModelTableExpr(`schedule.meal_plan_entries`)
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("meal plan postgres: insert day: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("meal plan postgres: count inserted rows: %w", err)
	}
	stats.Rows += inserted
	return stats, nil
}

func (s *Store) ClearDay(ctx context.Context, date domain.Date) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return deleteDay(ctx, db, tenantID, date)
}

func deleteDay(ctx context.Context, db bun.IDB, tenantID int64, date domain.Date) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	query := db.NewDelete().Model((*row)(nil)).
		ModelTableExpr(`schedule.meal_plan_entries AS "meal_plan_entry"`).
		Where(`"meal_plan_entry".tenant_id = ?`, tenantID).
		Where(`"meal_plan_entry".date = ?`, date)
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("meal plan postgres: delete day: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("meal plan postgres: count deleted rows: %w", err)
	}
	stats.Rows = rows
	return stats, nil
}
