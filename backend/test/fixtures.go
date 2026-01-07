package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Fixture helpers for hermetic testing. Each helper creates a real database record
// with proper relationships and returns the created entity with its real ID.
// Tests should call these to create test data, then defer cleanup.

// CreateTestActivityCategory creates a real activity category in the database
func CreateTestActivityCategory(tb testing.TB, db *bun.DB, name string) *activities.Category {
	tb.Helper()

	category := &activities.Category{
		Name:  name,
		Color: "#CCCCCC",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := db.NewInsert().
		Model(category).
		ModelTableExpr(`activities.categories`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test activity category")

	return category
}

// CreateTestActivityGroup creates a real activity group in the database
// Activity groups require a category, so this helper creates one automatically
func CreateTestActivityGroup(tb testing.TB, db *bun.DB, name string) *activities.Group {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First create a category (activities.groups.category_id is required)
	category := CreateTestActivityCategory(tb, db, fmt.Sprintf("Category-%s-%d", name, time.Now().UnixNano()))

	// Create the activity group
	group := &activities.Group{
		Name:            name,
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      category.ID,
	}

	err := db.NewInsert().
		Model(group).
		ModelTableExpr(`activities.groups AS "group"`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test activity group")

	return group
}

// CreateTestRoom creates a real room in the database
func CreateTestRoom(tb testing.TB, db *bun.DB, name string) *facilities.Room {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make room name unique by appending timestamp
	uniqueName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())

	room := &facilities.Room{
		Name:     uniqueName,
		Building: "Test Building",
		Capacity: intPtr(30),
	}

	err := db.NewInsert().
		Model(room).
		ModelTableExpr(`facilities.rooms`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test room")

	return room
}

// CreateTestDevice creates a real IoT device in the database
func CreateTestDevice(tb testing.TB, db *bun.DB, deviceID string) *iot.Device {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make device ID unique by appending timestamp if needed
	uniqueDeviceID := fmt.Sprintf("%s-%d", deviceID, time.Now().UnixNano())

	device := &iot.Device{
		DeviceID:   uniqueDeviceID,
		DeviceType: "rfid_reader",
		Name:       stringPtr("Test Device"),
		Status:     iot.DeviceStatusActive,
		APIKey:     stringPtr("test-api-key-" + uniqueDeviceID),
	}

	err := db.NewInsert().
		Model(device).
		ModelTableExpr(`iot.devices`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test device")

	return device
}

// CreateTestPerson creates a real person in the database (required for staff creation)
func CreateTestPerson(tb testing.TB, db *bun.DB, firstName, lastName string) *users.Person {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	person := &users.Person{
		FirstName: firstName,
		LastName:  lastName,
	}

	err := db.NewInsert().
		Model(person).
		ModelTableExpr(`users.persons`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test person")

	return person
}

// CreateTestStaff creates a real staff member in the database
// This requires a person, so it creates one automatically
func CreateTestStaff(tb testing.TB, db *bun.DB, firstName, lastName string) *users.Staff {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create person first
	person := CreateTestPerson(tb, db, firstName, lastName)

	// Create staff record
	staff := &users.Staff{
		PersonID: person.ID,
	}

	err := db.NewInsert().
		Model(staff).
		ModelTableExpr(`users.staff`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test staff")

	return staff
}

// CreateTestStudent creates a real student in the database
// This requires a person, so it creates one automatically
func CreateTestStudent(tb testing.TB, db *bun.DB, firstName, lastName, schoolClass string) *users.Student {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create person first (Student has FK to Person)
	person := CreateTestPerson(tb, db, firstName, lastName)

	// Create student record
	student := &users.Student{
		PersonID:    person.ID,
		SchoolClass: schoolClass,
	}

	err := db.NewInsert().
		Model(student).
		ModelTableExpr(`users.students`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test student")

	return student
}

// CleanupActivityFixtures removes activity-related test fixtures from the database.
// Pass activity group IDs, device IDs, room IDs, or any combination.
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

		// Delete from active.groups (if it exists)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.groups").
			Where("group_id = ? OR device_id = ?", id, id).
			Exec(ctx)

		// Delete from active.visits (cascade cleanup)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.visits").
			Where("active_group_id IN (SELECT id FROM active.groups WHERE group_id = ?)", id).
			Exec(ctx)

		// Delete from activities.groups
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("activities.groups").
			Where("id = ?", id).
			Exec(ctx)

		// Delete from activities.categories
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("activities.categories").
			Where("id = ?", id).
			Exec(ctx)

		// Delete from iot.devices
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("iot.devices").
			Where("id = ?", id).
			Exec(ctx)

		// Delete from facilities.rooms
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("facilities.rooms").
			Where("id = ?", id).
			Exec(ctx)

		// Delete from users.students
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.students").
			Where("id = ?", id).
			Exec(ctx)

		// Delete from users.staff
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.staff").
			Where("id = ?", id).
			Exec(ctx)

		// Delete from users.persons
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.persons").
			Where("id = ?", id).
			Exec(ctx)
	}
}

// Helper functions for pointer creation
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}
