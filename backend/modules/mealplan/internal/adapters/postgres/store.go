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

func (s *Store) FindWeek(ctx context.Context, start, end domain.Date) ([]domain.Entry, int64, error) {
	db, _, err := s.database(ctx)
	if err != nil {
		return nil, 0, err
	}
	var rows []row
	err = db.NewSelect().Model(&rows).
		ModelTableExpr(`schedule.meal_plan_entries AS "meal_plan_entry"`).
		Where(`"meal_plan_entry".date >= ?`, start).
		Where(`"meal_plan_entry".date <= ?`, end).
		OrderExpr(`"meal_plan_entry".date ASC`).
		OrderExpr(`"meal_plan_entry".position ASC`).
		Scan(ctx)
	if err != nil {
		return nil, 1, fmt.Errorf("meal plan postgres: find week: %w", err)
	}
	entries := make([]domain.Entry, 0, len(rows))
	for _, value := range rows {
		entries = append(entries, domain.Entry{Date: value.Date, Position: value.Position, Dish: value.Dish, Note: value.Note})
	}
	return entries, 1, nil
}

func (s *Store) ReplaceDay(ctx context.Context, date domain.Date, dishes []domain.Dish) (int64, int64, time.Duration, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	started := time.Now()
	deleted, err := deleteDay(ctx, db, date)
	lockWait := time.Since(started)
	if err != nil || len(dishes) == 0 {
		return deleted, 1, lockWait, err
	}
	rows := make([]row, 0, len(dishes))
	for position, dish := range dishes {
		rows = append(rows, row{TenantID: tenantID, Date: date, Position: position, Dish: dish.Dish, Note: dish.Note})
	}
	started = time.Now()
	result, err := db.NewInsert().Model(&rows).ModelTableExpr(`schedule.meal_plan_entries`).Exec(ctx)
	lockWait += time.Since(started)
	if err != nil {
		return deleted, 2, lockWait, fmt.Errorf("meal plan postgres: insert day: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return deleted, 2, lockWait, fmt.Errorf("meal plan postgres: count inserted rows: %w", err)
	}
	return deleted + inserted, 2, lockWait, nil
}

func (s *Store) ClearDay(ctx context.Context, date domain.Date) (int64, int64, time.Duration, error) {
	db, _, err := s.database(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	started := time.Now()
	rows, err := deleteDay(ctx, db, date)
	return rows, 1, time.Since(started), err
}

func deleteDay(ctx context.Context, db bun.IDB, date domain.Date) (int64, error) {
	result, err := db.NewDelete().Model((*row)(nil)).
		ModelTableExpr(`schedule.meal_plan_entries AS "meal_plan_entry"`).
		Where(`"meal_plan_entry".date = ?`, date).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("meal plan postgres: delete day: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("meal plan postgres: count deleted rows: %w", err)
	}
	return rows, nil
}
