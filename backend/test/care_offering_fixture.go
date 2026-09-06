package test

import (
	"context"
	"encoding/json"
	"testing"

	enrollment "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/uptrace/bun"
)

// InsertTestCareOffering inserts a minimal catalog fixture without relying on
// persistence tags on the legacy service value.
func InsertTestCareOffering(tb testing.TB, db *bun.DB, ctx context.Context, offering *enrollment.CareOffering) {
	tb.Helper()
	days, err := json.Marshal(offering.AvailableDays)
	if err != nil {
		tb.Fatalf("encode fixture offering days: %v", err)
	}
	err = db.NewRaw(`INSERT INTO enrollment.care_offerings
 (tenant_id, phase_id, name, days_of_week_mode, available_days, is_active, is_required, counts_as_care)
 VALUES (?, ?, ?, ?, ?::jsonb, ?, ?, ?) RETURNING id, created_at, updated_at, selection_rule, auto_add_grade_levels`,
		offering.TenantID, offering.PhaseID, offering.Name, offering.DaysOfWeekMode,
		string(days), offering.IsActive, offering.IsRequired, offering.CountsAsCare).Scan(ctx, offering)
	if err != nil {
		tb.Fatalf("create fixture care offering: %v", err)
	}
}
