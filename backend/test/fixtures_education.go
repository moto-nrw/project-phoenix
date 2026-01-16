package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Education Domain Fixtures
// ============================================================================

// CreateTestEducationGroup creates a real education group (Schulklasse) in the database.
// Note: This is different from CreateTestActivityGroup (activities.groups).
func CreateTestEducationGroup(tb testing.TB, db *bun.DB, name string) *education.Group {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make name unique by appending timestamp
	uniqueName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())

	group := &education.Group{
		Name: uniqueName,
	}

	err := db.NewInsert().
		Model(group).
		ModelTableExpr(`education.groups`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test education group")

	return group
}

// CreateTestTeacher creates a real teacher in the database.
// Teachers require a Staff record, which requires a Person record.
// Returns the teacher with Staff reference populated for cleanup.
func CreateTestTeacher(tb testing.TB, db *bun.DB, firstName, lastName string) *users.Teacher {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create staff first (which creates person)
	staff := CreateTestStaff(tb, db, firstName, lastName)

	teacher := &users.Teacher{
		StaffID: staff.ID,
	}

	err := db.NewInsert().
		Model(teacher).
		ModelTableExpr(`users.teachers`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test teacher")

	// Store staff reference for cleanup
	teacher.Staff = staff

	return teacher
}

// CreateTestGroupTeacher creates a group-teacher assignment in the database.
func CreateTestGroupTeacher(tb testing.TB, db *bun.DB, groupID, teacherID int64) *education.GroupTeacher {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gt := &education.GroupTeacher{
		GroupID:   groupID,
		TeacherID: teacherID,
	}

	err := db.NewInsert().
		Model(gt).
		ModelTableExpr(`education.group_teacher`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test group teacher assignment")

	return gt
}

// ============================================================================
// Active Domain Fixtures (Sessions and Visits)
// ============================================================================

// CreateTestActiveGroup creates a real active group (session) in the database.
// This requires an ActivityGroup (activities.groups) and Room to exist.
// Use this for testing session management and visit tracking.
func CreateTestActiveGroup(tb testing.TB, db *bun.DB, activityGroupID, roomID int64) *active.Group {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now()
	activeGroup := &active.Group{
		GroupID:        activityGroupID,
		RoomID:         roomID,
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
	}

	err := db.NewInsert().
		Model(activeGroup).
		ModelTableExpr(`active.groups`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test active group")

	return activeGroup
}

// CreateTestVisit creates a real visit record in the database.
// This requires a Student and ActiveGroup to already exist.
func CreateTestVisit(tb testing.TB, db *bun.DB, studentID, activeGroupID int64, entryTime time.Time, exitTime *time.Time) *active.Visit {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	visit := &active.Visit{
		StudentID:     studentID,
		ActiveGroupID: activeGroupID,
		EntryTime:     entryTime,
		ExitTime:      exitTime,
	}

	err := db.NewInsert().
		Model(visit).
		ModelTableExpr(`active.visits`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test visit")

	return visit
}

// CreateTestGroupSupervisor creates a real group supervisor record in the database.
// This requires a Staff and ActiveGroup to already exist.
func CreateTestGroupSupervisor(tb testing.TB, db *bun.DB, staffID, activeGroupID int64, role string) *active.GroupSupervisor {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	supervisor := &active.GroupSupervisor{
		StaffID:   staffID,
		GroupID:   activeGroupID,
		Role:      role,
		StartDate: time.Now(),
	}

	err := db.NewInsert().
		Model(supervisor).
		ModelTableExpr(`active.group_supervisors`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test group supervisor")

	return supervisor
}

// Helper functions for pointer creation
// CleanupPerson removes a person from the database by ID.
func CleanupPerson(tb testing.TB, db *bun.DB, personID int64) {
	tb.Helper()

	cleanupDelete(tb, db.NewDelete().
		Model((*interface{})(nil)).
		Table(tableUsersPersons).
		Where(whereIDEquals, personID),
		tableUsersPersons)
}

// CleanupAccount removes an account and related auth records from the database.
func CleanupAccount(tb testing.TB, db *bun.DB, accountID int64) {
	tb.Helper()

	CleanupAuthFixtures(tb, db, accountID)
}

// CleanupStaffFixtures removes staff fixtures from the database.
// Pass a staff ID and it will clean up the staff, person, and any related records.
// If the staff has an account, call CleanupAuthFixtures separately with the account ID.
func CleanupStaffFixtures(tb testing.TB, db *bun.DB, staffIDs ...int64) {
	tb.Helper()

	if len(staffIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, staffID := range staffIDs {
		// First get the staff to find the person ID
		var staff struct {
			PersonID int64 `bun:"person_id"`
		}
		_ = db.NewSelect().
			Model(&staff).
			Table(tableUsersStaff).
			Column("person_id").
			Where(whereIDEquals, staffID).
			Scan(ctx)

		// Delete teacher if exists (depends on staff)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersTeachers).
			Where("staff_id = ?", staffID),
			tableUsersTeachers)

		// Delete staff
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersStaff).
			Where(whereIDEquals, staffID),
			tableUsersStaff)

		// Delete person if we found one
		if staff.PersonID > 0 {
			cleanupDelete(tb, db.NewDelete().
				Model((*interface{})(nil)).
				Table(tableUsersPersons).
				Where(whereIDEquals, staff.PersonID),
				tableUsersPersons)
		}
	}
}

// CleanupTeacherFixtures removes teacher fixtures from the database.
// Pass a teacher ID and it will clean up the full chain: teacher -> staff -> person.
// Also cleans up the associated account via CleanupAuthFixtures.
func CleanupTeacherFixtures(tb testing.TB, db *bun.DB, teacherIDs ...int64) {
	tb.Helper()

	if len(teacherIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, teacherID := range teacherIDs {
		// Get the teacher to find the staff ID
		var teacher struct {
			StaffID int64 `bun:"staff_id"`
		}
		_ = db.NewSelect().
			Model(&teacher).
			Table(tableUsersTeachers).
			Column("staff_id").
			Where(whereIDEquals, teacherID).
			Scan(ctx)

		// Get the staff to find the person ID and account ID
		var staff struct {
			PersonID int64 `bun:"person_id"`
		}
		_ = db.NewSelect().
			Model(&staff).
			Table(tableUsersStaff).
			Column("person_id").
			Where(whereIDEquals, teacher.StaffID).
			Scan(ctx)

		// Get the person to find the account ID
		var person struct {
			AccountID *int64 `bun:"account_id"`
		}
		_ = db.NewSelect().
			Model(&person).
			Table(tableUsersPersons).
			Column("account_id").
			Where(whereIDEquals, staff.PersonID).
			Scan(ctx)

		// Delete teacher
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersTeachers).
			Where(whereIDEquals, teacherID),
			tableUsersTeachers)

		// Delete staff
		if teacher.StaffID > 0 {
			cleanupDelete(tb, db.NewDelete().
				Model((*interface{})(nil)).
				Table(tableUsersStaff).
				Where(whereIDEquals, teacher.StaffID),
				tableUsersStaff)
		}

		// Delete person
		if staff.PersonID > 0 {
			cleanupDelete(tb, db.NewDelete().
				Model((*interface{})(nil)).
				Table(tableUsersPersons).
				Where(whereIDEquals, staff.PersonID),
				tableUsersPersons)
		}

		// Delete account if exists
		if person.AccountID != nil && *person.AccountID > 0 {
			CleanupAuthFixtures(tb, db, *person.AccountID)
		}
	}
}

// CreateTestPersonWithAccountID creates a person linked to an existing account ID.
// Use this when you already have an account and want to link a person to it.
func CreateTestPersonWithAccountID(tb testing.TB, db *bun.DB, firstName, lastName string, accountID int64) *users.Person {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	person := &users.Person{
		FirstName: firstName,
		LastName:  lastName,
		AccountID: &accountID,
	}

	err := db.NewInsert().
		Model(person).
		ModelTableExpr(`users.persons`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test person with account ID")

	return person
}
