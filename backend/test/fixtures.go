package test

import (
	"context"
	"testing"

	"github.com/uptrace/bun"
)

// SQL constants to avoid duplication across fixture files
const (
	whereAccountIDIn    = "account_id IN (?)"
	tableUsersTeachers  = "users.teachers"
	tableUsersStaff     = "users.staff"
	tableUsersPersons   = "users.persons"
	tableActiveVisits   = "active.visits"
	tableUsersRFIDCards = "users.rfid_cards"
)

// cleanupDelete executes a delete query and logs any unexpected errors.
// This provides visibility into cleanup failures without causing test failures.
// Expected errors (like "Model(nil interface)" from BUN) are silently ignored.
func cleanupDelete(tb testing.TB, query *bun.DeleteQuery, table string) {
	_, err := query.Exec(context.Background())
	if err != nil {
		// Filter out expected BUN errors from using nil model
		errStr := err.Error()
		if errStr == "bun: Model(nil interface *interface {})" ||
			errStr == "bun: Model(nil)" {
			return
		}
		tb.Logf("cleanup %s: %v", table, err)
	}
}

// Fixture helpers for hermetic testing. Each helper creates a real database record
// with proper relationships and returns the created entity with its real ID.
// Tests should call these to create test data, then defer cleanup.
//
// Fixture creation functions are organized into domain-specific files:
//   - fixtures_activities.go: Activity categories and groups
//   - fixtures_active.go: Attendance records
//   - fixtures_facilities.go: Rooms
//   - fixtures_iot.go: IoT devices
//   - fixtures_users.go: Persons, staff, students
//   - fixtures_cleanup.go: Cleanup utilities
//   - fixtures_auth.go: Accounts, roles, permissions, tokens (in separate file)
//   - fixtures_education.go: Education groups, teachers (in separate file)
