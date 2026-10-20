package test

import (
	"context"
	"testing"

	"github.com/uptrace/bun"
)

// RestoreStudentSupervisorNotesColumn re-adds the „Betreuernotizen" column
// that migration 1.15.458 dropped (#4268), empty, inside this test's isolated
// database. Contracts written for the migrations before it describe a care
// profile that still has the column.
func RestoreStudentSupervisorNotesColumn(tb testing.TB, db *bun.DB) {
	tb.Helper()
	if db == nil {
		tb.Fatal("restore supervisor notes column: database is required")
	}
	entry, isolated := isolatedTestDatabases.Load(topLevelTestName(tb))
	if !isolated || entry.(*isolatedTestDatabase).db != db {
		tb.Fatal("restore supervisor notes column requires this test's isolated database")
	}
	if _, err := db.ExecContext(context.Background(),
		`ALTER TABLE users.student_care_profiles ADD COLUMN IF NOT EXISTS supervisor_notes TEXT`); err != nil {
		tb.Fatalf("restore users.student_care_profiles.supervisor_notes: %v", err)
	}
}
