package test

import (
	"fmt"
	"testing"

	"github.com/uptrace/bun"
)

// ============================================================================
// Cleanup Fixtures
// ============================================================================

// CleanupActivityFixtures removes activity-related and education-related test fixtures from the database.
// Pass activity group IDs, device IDs, room IDs, education group IDs, teacher IDs, or any combination.
// This is typically called in a defer statement to ensure cleanup happens.
//
// Example:
//
//	activity := CreateTestActivityGroup(t, db, "Test")
//	device := CreateTestDevice(t, db, "dev-001")
//	room := CreateTestRoom(t, db, "Room 1")
//	defer CleanupActivityFixtures(t, db, activity.ID, device.ID, room.ID)
func CleanupActivityFixtures(tb testing.TB, db *bun.DB, ids ...int64) {
	tb.Helper()

	if len(ids) == 0 {
		return
	}

	// Batch delete all fixtures matching the IDs
	// This is a simple approach that deletes from any table with these IDs
	// More sophisticated cleanup could track which table each ID belongs to

	for _, id := range ids {
		// Try to delete from each table type
		// Errors are logged but don't fail tests since we don't know which table each ID belongs to

		// ========================================
		// Education domain cleanup (FK-dependent order)
		// ========================================

		// Delete from education.group_substitution (depends on group and staff)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("education.group_substitution").
			Where("group_id = ? OR regular_staff_id = ? OR substitute_staff_id = ?", id, id, id),
			"education.group_substitution")

		// Delete from education.group_teacher (depends on group and teacher)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("education.group_teacher").
			Where("group_id = ? OR teacher_id = ?", id, id),
			"education.group_teacher")

		// Delete from users.teachers (depends on staff)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersTeachers).
			Where("id = ? OR staff_id = ?", id, id),
			tableUsersTeachers)

		// Delete from education.groups
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("education.groups").
			Where(whereIDEquals, id),
			"education.groups")

		// ========================================
		// Active domain cleanup
		// ========================================

		// Delete from active.visits by direct ID, by student_id, or by active_group_id
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableActiveVisits).
			Where("id = ? OR student_id = ? OR active_group_id = ?", id, id, id),
			tableActiveVisits)

		// Delete from active.visits (cascade cleanup via activities.groups reference)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableActiveVisits).
			Where("active_group_id IN (SELECT id FROM active.groups WHERE group_id = ?)", id),
			"active.visits (cascade)")

		// Delete from active.groups by direct ID or by reference
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.groups").
			Where("id = ? OR group_id = ? OR device_id = ?", id, id, id),
			"active.groups")

		// ========================================
		// Activities domain cleanup
		// ========================================

		// Delete from activities.groups
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("activities.groups").
			Where(whereIDEquals, id),
			"activities.groups")

		// Delete from activities.categories
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("activities.categories").
			Where(whereIDEquals, id),
			"activities.categories")

		// ========================================
		// IoT domain cleanup
		// ========================================

		// Delete from iot.devices
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("iot.devices").
			Where(whereIDEquals, id),
			"iot.devices")

		// ========================================
		// Facilities domain cleanup
		// ========================================

		// Delete from facilities.rooms
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("facilities.rooms").
			Where(whereIDEquals, id),
			"facilities.rooms")

		// ========================================
		// Users domain cleanup (FK-dependent order)
		// ========================================

		// Delete from users.guests (depends on staff)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.guests").
			Where("id = ? OR staff_id = ?", id, id),
			"users.guests")

		// Delete from users.profiles (depends on account)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.profiles").
			Where(whereIDOrAccountID, id, id),
			"users.profiles")

		// Delete from active.attendance (by student_id before deleting student)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.attendance").
			Where("student_id = ?", id),
			"active.attendance")

		// Delete from users.students
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.students").
			Where(whereIDEquals, id),
			"users.students")

		// Delete from users.staff
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersStaff).
			Where(whereIDEquals, id),
			tableUsersStaff)

		// Delete from users.persons (last, as it's referenced by students and staff)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersPersons).
			Where(whereIDEquals, id),
			tableUsersPersons)

		// ========================================
		// Active domain cleanup (continued)
		// ========================================

		// Delete from active.group_supervisors
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.group_supervisors").
			Where("id = ? OR staff_id = ? OR group_id = ?", id, id, id),
			"active.group_supervisors")

		// NOTE: Auth domain cleanup intentionally omitted here.
		// Use CleanupAuthFixtures(accountIDs...) for auth cleanup.
		// Reason: Using generic IDs against auth tables causes cross-domain
		// collisions (e.g., student ID 5 would delete role ID 5).

		// ========================================
		// Users domain extended cleanup
		// ========================================

		// Delete from users.privacy_consents (by student_id)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.privacy_consents").
			Where("id = ? OR student_id = ?", id, id),
			"users.privacy_consents")

		// Delete from users.persons_guardians (by person_id or guardian_account_id)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.persons_guardians").
			Where("id = ? OR person_id = ? OR guardian_account_id = ?", id, id, id),
			"users.persons_guardians")

		// Delete from users.guardian_profiles
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.guardian_profiles").
			Where(whereIDEquals, id),
			"users.guardian_profiles")

		// Delete from users.rfid_cards (note: string ID, but try as int64)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersRFIDCards).
			Where(whereIDEquals, fmt.Sprintf("%d", id)),
			tableUsersRFIDCards)
	}
}

// CleanupAuthFixtures removes auth account fixtures and their related records.
// Pass account IDs only - this will cascade delete:
//   - auth.tokens (by account_id)
//   - auth.account_roles (by account_id)
//   - auth.account_permissions (by account_id)
//   - auth.accounts (by id)
//
// NOTE: This does NOT touch auth.roles, auth.permissions, or auth.role_permissions
// since those are not account-specific. Use CleanupTableRecords for those if needed.
func CleanupAuthFixtures(tb testing.TB, db *bun.DB, accountIDs ...int64) {
	tb.Helper()

	if len(accountIDs) == 0 {
		return
	}

	// Use IN clause for efficiency instead of loop
	// Delete tokens first (depends on accounts)
	cleanupDelete(tb, db.NewDelete().
		Model((*any)(nil)).
		Table("auth.tokens").
		Where(whereAccountIDIn, bun.In(accountIDs)),
		"auth.tokens")

	// Delete account_roles (by account_id only - never by role_id!)
	cleanupDelete(tb, db.NewDelete().
		Model((*any)(nil)).
		Table("auth.account_roles").
		Where(whereAccountIDIn, bun.In(accountIDs)),
		"auth.account_roles")

	// Delete account_permissions (by account_id only - never by permission_id!)
	cleanupDelete(tb, db.NewDelete().
		Model((*any)(nil)).
		Table("auth.account_permissions").
		Where(whereAccountIDIn, bun.In(accountIDs)),
		"auth.account_permissions")

	// Finally delete the accounts themselves
	cleanupDelete(tb, db.NewDelete().
		Model((*any)(nil)).
		Table("auth.accounts").
		Where("id IN (?)", bun.In(accountIDs)),
		"auth.accounts")
}

// CleanupParentAccountFixtures removes parent accounts by their IDs.
func CleanupParentAccountFixtures(tb testing.TB, db *bun.DB, accountIDs ...int64) {
	tb.Helper()

	if len(accountIDs) == 0 {
		return
	}

	cleanupDelete(tb, db.NewDelete().
		Model((*any)(nil)).
		Table("auth.accounts_parents").
		Where("id IN (?)", bun.In(accountIDs)),
		"auth.accounts_parents")
}

// CleanupRFIDCards removes RFID cards by their string IDs.
func CleanupRFIDCards(tb testing.TB, db *bun.DB, tagIDs ...string) {
	tb.Helper()

	if len(tagIDs) == 0 {
		return
	}

	for _, tagID := range tagIDs {
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersRFIDCards).
			Where(whereIDEquals, tagID),
			tableUsersRFIDCards)
	}
}
