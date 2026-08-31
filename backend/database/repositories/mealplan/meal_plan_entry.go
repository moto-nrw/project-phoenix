package mealplan

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/mealplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const tableExprMealPlanEntriesAsEntry = `schedule.meal_plan_entries AS "meal_plan_entry"`

type MealPlanEntryRepository struct {
	*base.Repository[*mealplan.MealPlanEntry]
	db *bun.DB
}

func NewMealPlanEntryRepository(db *bun.DB) mealplan.MealPlanEntryRepository {
	repo := base.NewRepository[*mealplan.MealPlanEntry](db, "schedule.meal_plan_entries", "MealPlanEntry")
	repo.TenantScoped = true
	return &MealPlanEntryRepository{Repository: repo, db: db}
}

func (r *MealPlanEntryRepository) FindByDateRange(ctx context.Context, start, end timezone.Date) ([]*mealplan.MealPlanEntry, error) {
	var entries []*mealplan.MealPlanEntry
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&entries).
		ModelTableExpr(tableExprMealPlanEntriesAsEntry).
		Where(`"meal_plan_entry".date >= ?`, start).
		Where(`"meal_plan_entry".date <= ?`, end).
		OrderExpr(`"meal_plan_entry".date ASC`).
		OrderExpr(`"meal_plan_entry".position ASC`)

	query = base.WithTenantFilter(ctx, query, "meal_plan_entry")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find meal plan entries by date range", Err: base.TranslateNotFound(err)}
	}
	return entries, nil
}

func (r *MealPlanEntryRepository) ReplaceDay(ctx context.Context, date timezone.Date, entries []*mealplan.MealPlanEntry) error {
	if date.IsZero() {
		return fmt.Errorf("meal plan entry date is required")
	}
	tid := tenant.FromContext(ctx)

	// Clear the day first, then re-insert. The caller runs us inside a tenant
	// transaction, so the delete+insert is atomic.
	if err := r.DeleteByDate(ctx, date); err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}

	for _, entry := range entries {
		if entry.GetTenantID() == 0 && tid != 0 {
			entry.SetTenantID(tid)
		}
		entry.Date = date
	}

	if _, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&entries).
		ModelTableExpr("schedule.meal_plan_entries").
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "insert meal plan entries", Err: base.TranslateNotFound(err)}
	}
	return nil
}

func (r *MealPlanEntryRepository) DeleteByDate(ctx context.Context, date timezone.Date) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*mealplan.MealPlanEntry)(nil)).
		ModelTableExpr("schedule.meal_plan_entries").
		Where("date = ?", date)

	if tid := tenant.FromContext(ctx); tid != 0 {
		query = query.Where("tenant_id = ?", tid)
	}

	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "delete meal plan entry by date", Err: base.TranslateNotFound(err)}
	}
	return nil
}
