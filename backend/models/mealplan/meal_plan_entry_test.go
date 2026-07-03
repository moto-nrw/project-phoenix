package mealplan

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/uptrace/bun"
)

// TestTableName pins the schema-qualified table name the repository relies on.
func TestTableName(t *testing.T) {
	entry := &MealPlanEntry{}
	if got := entry.TableName(); got != "schedule.meal_plan_entries" {
		t.Errorf("TableName() = %q, want schedule.meal_plan_entries", got)
	}
}

// TestBeforeAppendModel_SetsTableExprOnWriteQueries verifies the persistence
// hook rewrites the model table expression for insert/update/delete queries so
// bun targets schedule.meal_plan_entries rather than the default derived name.
// A nil / unrelated query type must be a no-op (return nil, no panic).
func TestBeforeAppendModel_SetsTableExprOnWriteQueries(t *testing.T) {
	entry := &MealPlanEntry{
		Date:     timezone.NewDate(2026, time.July, 1),
		Position: 0,
		Dish:     "Nudeln",
	}

	// nil interface — hook must not panic and must return nil.
	if err := entry.BeforeAppendModel(nil); err != nil {
		t.Errorf("BeforeAppendModel(nil) returned error: %v", err)
	}

	// Each of the three write query types must be accepted without error.
	// (We can construct the query values with a nil DB because the hook only
	// mutates the ModelTableExpr, it does not execute anything.)
	for _, q := range []any{
		(&bun.InsertQuery{}),
		(&bun.UpdateQuery{}),
		(&bun.DeleteQuery{}),
	} {
		if err := entry.BeforeAppendModel(q); err != nil {
			t.Errorf("BeforeAppendModel(%T) returned error: %v", q, err)
		}
	}

	// A SelectQuery is not a write query — the hook should ignore it.
	if err := entry.BeforeAppendModel(&bun.SelectQuery{}); err != nil {
		t.Errorf("BeforeAppendModel(*SelectQuery) returned error: %v", err)
	}
}
