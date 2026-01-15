package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/uptrace/bun"
)

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Batch delete all fixtures matching the IDs
	// This is a simple approach that deletes from any table with these IDs
	// More sophisticated cleanup could track which table each ID belongs to

	for _, id := range ids {
		// Try to delete from each table type
		// Ignore errors since we don't know which table each ID belongs to

		// ========================================
		// Education domain cleanup (FK-dependent order)
		// ========================================

		// Delete from education.group_substitution (depends on group and staff)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("education.group_substitution").
			Where("group_id = ? OR regular_staff_id = ? OR substitute_staff_id = ?", id, id, id).
			Exec(ctx)

		// Delete from education.group_teacher (depends on group and teacher)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("education.group_teacher").
			Where("group_id = ? OR teacher_id = ?", id, id).
			Exec(ctx)

		// Delete from users.teachers (depends on staff)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.teachers").
			Where("id = ? OR staff_id = ?", id, id).
			Exec(ctx)

		// Delete from education.groups
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("education.groups").
			Where(whereIDEquals, id).
			Exec(ctx)

		// ========================================
		// Active domain cleanup
		// ========================================

		// Delete from active.visits by direct ID, by student_id, or by active_group_id
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.visits").
			Where("id = ? OR student_id = ? OR active_group_id = ?", id, id, id).
			Exec(ctx)

		// Delete from active.visits (cascade cleanup via activities.groups reference)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.visits").
			Where("active_group_id IN (SELECT id FROM active.groups WHERE group_id = ?)", id).
			Exec(ctx)

		// Delete from active.groups by direct ID or by reference
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.groups").
			Where("id = ? OR group_id = ? OR device_id = ?", id, id, id).
			Exec(ctx)

		// ========================================
		// Activities domain cleanup
		// ========================================

		// Delete from activities.groups
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("activities.groups").
			Where(whereIDEquals, id).
			Exec(ctx)

		// Delete from activities.categories
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("activities.categories").
			Where(whereIDEquals, id).
			Exec(ctx)

		// ========================================
		// IoT domain cleanup
		// ========================================

		// Delete from iot.devices
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("iot.devices").
			Where(whereIDEquals, id).
			Exec(ctx)

		// ========================================
		// Facilities domain cleanup
		// ========================================

		// Delete from facilities.rooms
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("facilities.rooms").
			Where(whereIDEquals, id).
			Exec(ctx)

		// ========================================
		// Users domain cleanup (FK-dependent order)
		// ========================================

		// Delete from users.guests (depends on staff)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.guests").
			Where("id = ? OR staff_id = ?", id, id).
			Exec(ctx)

		// Delete from users.profiles (depends on account)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.profiles").
			Where(whereIDOrAccountID, id, id).
			Exec(ctx)

		// Delete from active.attendance (by student_id before deleting student)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.attendance").
			Where("student_id = ?", id).
			Exec(ctx)

		// Delete from users.students
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.students").
			Where(whereIDEquals, id).
			Exec(ctx)

		// Delete from users.staff
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.staff").
			Where(whereIDEquals, id).
			Exec(ctx)

		// Delete from users.persons (last, as it's referenced by students and staff)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.persons").
			Where(whereIDEquals, id).
			Exec(ctx)

		// ========================================
		// Active domain cleanup (continued)
		// ========================================

		// Delete from active.group_supervisors
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.group_supervisors").
			Where("id = ? OR staff_id = ? OR group_id = ?", id, id, id).
			Exec(ctx)

		// NOTE: Auth domain cleanup intentionally omitted here.
		// Use CleanupAuthFixtures(accountIDs...) for auth cleanup.
		// Reason: Using generic IDs against auth tables causes cross-domain
		// collisions (e.g., student ID 5 would delete role ID 5).

		// ========================================
		// Users domain extended cleanup
		// ========================================

		// Delete from users.privacy_consents (by student_id)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.privacy_consents").
			Where("id = ? OR student_id = ?", id, id).
			Exec(ctx)

		// Delete from users.persons_guardians (by person_id or guardian_account_id)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.persons_guardians").
			Where("id = ? OR person_id = ? OR guardian_account_id = ?", id, id, id).
			Exec(ctx)

		// Delete from users.guardian_profiles
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.guardian_profiles").
			Where(whereIDEquals, id).
			Exec(ctx)

		// Delete from users.rfid_cards (note: string ID, but try as int64)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.rfid_cards").
			Where(whereIDEquals, fmt.Sprintf("%d", id)).
			Exec(ctx)
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use IN clause for efficiency instead of loop
	// Delete tokens first (depends on accounts)
	_, _ = db.NewDelete().
		Model((*any)(nil)).
		Table("auth.tokens").
		Where("account_id IN (?)", bun.In(accountIDs)).
		Exec(ctx)

	// Delete account_roles (by account_id only - never by role_id!)
	_, _ = db.NewDelete().
		Model((*any)(nil)).
		Table("auth.account_roles").
		Where("account_id IN (?)", bun.In(accountIDs)).
		Exec(ctx)

	// Delete account_permissions (by account_id only - never by permission_id!)
	_, _ = db.NewDelete().
		Model((*any)(nil)).
		Table("auth.account_permissions").
		Where("account_id IN (?)", bun.In(accountIDs)).
		Exec(ctx)

	// Finally delete the accounts themselves
	_, _ = db.NewDelete().
		Model((*any)(nil)).
		Table("auth.accounts").
		Where("id IN (?)", bun.In(accountIDs)).
		Exec(ctx)
}

// CleanupParentAccountFixtures removes parent accounts by their IDs.
func CleanupParentAccountFixtures(tb testing.TB, db *bun.DB, accountIDs ...int64) {
	tb.Helper()

	if len(accountIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = db.NewDelete().
		Model((*any)(nil)).
		Table("auth.accounts_parents").
		Where("id IN (?)", bun.In(accountIDs)).
		Exec(ctx)
}

// CleanupRFIDCards removes RFID cards by their string IDs.
func CleanupRFIDCards(tb testing.TB, db *bun.DB, tagIDs ...string) {
	tb.Helper()

	if len(tagIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, tagID := range tagIDs {
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.rfid_cards").
			Where(whereIDEquals, tagID).
			Exec(ctx)
	}
}
