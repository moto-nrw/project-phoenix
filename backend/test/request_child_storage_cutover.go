package test

import (
	"context"
	_ "embed"
	"testing"

	"github.com/uptrace/bun"
)

//go:embed testdata/request_child_offerings_before_cutover.sql
var requestChildOfferingsBeforeCutover string

// RestoreRequestChildStorageBeforeCutover recreates enrollment.request_child_offerings
// as the authoritative base table it was before migration 1.15.385 (#2714).
//
// Contracts written for the Expand (#2712), Backfill (#2713) and Cutover (#2714)
// migrations describe a world in which that table holds the rows. The cutover
// replaced it with a rollback-only view, and the Contract (#2719) removed the
// view and its archive, so those tests restore their world inside their own
// disposable clone first. Production never reverses either migration.
//
// It requires a per-test database (SetupIsolatedTestDB): it changes the schema
// and empties both owner tables and the backfill checkpoints, which is exactly
// the post-Expand state the callers expect.
func RestoreRequestChildStorageBeforeCutover(tb testing.TB, db *bun.DB) {
	tb.Helper()
	if db == nil {
		tb.Fatal("restore request child storage before cutover: database is required")
	}
	entry, isolated := isolatedTestDatabases.Load(topLevelTestName(tb))
	if !isolated || entry.(*isolatedTestDatabase).db != db {
		tb.Fatal("restore request child storage before cutover requires this test's isolated database")
	}
	var contracted bool
	if err := db.NewRaw(`SELECT to_regclass('enrollment.request_child_offerings') IS NULL
		AND to_regclass('enrollment.request_child_offerings_legacy') IS NULL
		AND to_regclass('enrollment.request_child_compatibility_reads') IS NULL`).
		Scan(context.Background(), &contracted); err != nil {
		tb.Fatalf("restore request child storage before cutover: inspect schema: %v", err)
	}
	if !contracted {
		tb.Fatal("restore request child storage before cutover requires the contracted schema")
	}
	if _, err := db.ExecContext(context.Background(), requestChildOfferingsBeforeCutover+`
		TRUNCATE enrollment.care_offering_bookings, enrollment.request_child_offering_selections;
		DELETE FROM enrollment.request_child_storage_backfill_checkpoints;
	`); err != nil {
		tb.Fatalf("restore request child storage before cutover: %v", err)
	}
}
