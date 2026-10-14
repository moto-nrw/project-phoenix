package test

import (
	"context"
	"testing"

	"github.com/uptrace/bun"
)

// InsertTestCareOffering inserts a catalog fixture row. An unset selection
// rule or auto-add grade list takes the column default.
func InsertTestCareOffering(tb testing.TB, db *bun.DB, ctx context.Context, offering *CareOffering) {
	tb.Helper()
	err := db.NewInsert().Model(offering).
		Returning("id, created_at, updated_at, selection_rule, auto_add_grade_levels").
		Scan(ctx)
	if err != nil {
		tb.Fatalf("create fixture care offering: %v", err)
	}
}
