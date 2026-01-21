package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Multi-Tenancy Test Fixtures
// ============================================================================
//
// These fixtures support testing RLS (Row-Level Security) without requiring
// the full BetterAuth organization setup. They use simple OGS IDs that can
// be generated and cleaned up independently.

// GenerateTestOGSID creates a unique OGS ID for testing.
// This is used as the ogs_id value for RLS filtering.
func GenerateTestOGSID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// CreateTestPersonWithOGS creates a person with an assigned ogs_id.
// This is essential for multi-tenancy testing where data must be scoped to an organization.
// Returns the person ID; returns 0 on error (logged but not fatal to allow cleanup to continue).
func CreateTestPersonWithOGS(tb testing.TB, db *bun.DB, firstName, lastName string, ogsID string) int64 {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var personID int64
	err := db.NewRaw(`
		INSERT INTO users.persons (first_name, last_name, ogs_id)
		VALUES (?, ?, ?)
		RETURNING id
	`, firstName, lastName, ogsID).Scan(ctx, &personID)
	if err != nil {
		tb.Fatalf("Failed to create test person with OGS: %v", err)
		return 0
	}

	return personID
}

// CreateTestStudentWithOGS creates a student with an assigned ogs_id.
// Creates both Person and Student records with the same ogs_id.
func CreateTestStudentWithOGS(tb testing.TB, db *bun.DB, firstName, lastName, schoolClass string, ogsID string) int64 {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create person first
	personID := CreateTestPersonWithOGS(tb, db, firstName, lastName, ogsID)

	// Create student
	var studentID int64
	err := db.NewRaw(`
		INSERT INTO users.students (person_id, school_class, ogs_id)
		VALUES (?, ?, ?)
		RETURNING id
	`, personID, schoolClass, ogsID).Scan(ctx, &studentID)
	require.NoError(tb, err, "Failed to create test student with OGS")

	return studentID
}

// CreateTestStaffWithOGS creates a staff member with an assigned ogs_id.
// Creates both Person and Staff records with the same ogs_id.
func CreateTestStaffWithOGS(tb testing.TB, db *bun.DB, firstName, lastName string, ogsID string) int64 {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create person first
	personID := CreateTestPersonWithOGS(tb, db, firstName, lastName, ogsID)

	// Create staff
	var staffID int64
	err := db.NewRaw(`
		INSERT INTO users.staff (person_id, ogs_id)
		VALUES (?, ?)
		RETURNING id
	`, personID, ogsID).Scan(ctx, &staffID)
	require.NoError(tb, err, "Failed to create test staff with OGS")

	return staffID
}

// CreateTestGroupWithOGS creates an education group with an assigned ogs_id.
func CreateTestGroupWithOGS(tb testing.TB, db *bun.DB, name string, ogsID string) int64 {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var groupID int64
	err := db.NewRaw(`
		INSERT INTO education.groups (name, ogs_id)
		VALUES (?, ?)
		RETURNING id
	`, fmt.Sprintf("%s-%d", name, time.Now().UnixNano()), ogsID).Scan(ctx, &groupID)
	require.NoError(tb, err, "Failed to create test education group with OGS")

	return groupID
}

// CreateTestRoomWithOGS creates a room with an assigned ogs_id.
func CreateTestRoomWithOGS(tb testing.TB, db *bun.DB, name string, ogsID string) int64 {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var roomID int64
	err := db.NewRaw(`
		INSERT INTO facilities.rooms (name, building, ogs_id)
		VALUES (?, ?, ?)
		RETURNING id
	`, fmt.Sprintf("%s-%d", name, time.Now().UnixNano()), "Test Building", ogsID).Scan(ctx, &roomID)
	require.NoError(tb, err, "Failed to create test room with OGS")

	return roomID
}

// CreateTestDeviceWithOGS creates an IoT device with an assigned ogs_id.
func CreateTestDeviceWithOGS(tb testing.TB, db *bun.DB, deviceID string, ogsID string) int64 {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var id int64
	uniqueDeviceID := fmt.Sprintf("%s-%d", deviceID, time.Now().UnixNano())
	err := db.NewRaw(`
		INSERT INTO iot.devices (device_id, device_type, name, status, api_key, ogs_id)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id
	`, uniqueDeviceID, "rfid_reader", "Test Device", "active", "test-api-key-"+uniqueDeviceID, ogsID).Scan(ctx, &id)
	require.NoError(tb, err, "Failed to create test device with OGS")

	return id
}

// SetRLSContext sets the RLS context for a database connection.
// This simulates what the tenant middleware does for authenticated requests.
// IMPORTANT: Use within a transaction for SET LOCAL to take effect.
// Note: SET LOCAL doesn't support parameterized queries, so we use fmt.Sprintf.
// This is safe because ogsID is controlled by test code.
func SetRLSContext(ctx context.Context, db bun.IDB, ogsID string) error {
	// SET LOCAL doesn't support $1 parameters - must use string interpolation
	// The value is wrapped in single quotes for PostgreSQL string literal
	query := fmt.Sprintf("SET LOCAL app.ogs_id = '%s'", ogsID)
	_, err := db.ExecContext(ctx, query)
	return err
}

// SetRLSContextWithRole sets both the RLS context AND assumes the test_user role.
// This is necessary because the postgres superuser bypasses RLS even with FORCE ROW LEVEL SECURITY.
// The test_user role must exist in the database (created by WP9 setup).
// IMPORTANT: Use within a transaction for SET LOCAL and SET ROLE to take effect.
func SetRLSContextWithRole(ctx context.Context, db bun.IDB, ogsID string) error {
	// First, assume the test_user role (non-superuser, doesn't bypass RLS)
	_, err := db.ExecContext(ctx, "SET LOCAL ROLE test_user")
	if err != nil {
		return fmt.Errorf("failed to set role: %w", err)
	}

	// Then set the ogs_id context
	query := fmt.Sprintf("SET LOCAL app.ogs_id = '%s'", ogsID)
	_, err = db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to set ogs_id: %w", err)
	}
	return nil
}

// ============================================================================
// Cleanup Helpers for Multi-Tenancy
// ============================================================================

// CleanupOrganization removes a test organization and related data.
func CleanupOrganization(tb testing.TB, db *bun.DB, orgID string) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Delete organization
	_, _ = db.NewRaw(`DELETE FROM public.organization WHERE id = ?`, orgID).Exec(ctx)
}

// CleanupTraeger removes a test Träger and related data.
func CleanupTraeger(tb testing.TB, db *bun.DB, traegerID string) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Delete in order: organizations -> bueros -> traeger
	_, _ = db.NewRaw(`DELETE FROM public.organization WHERE "traegerId" = ?`, traegerID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM tenant.buero WHERE traeger_id = ?`, traegerID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM tenant.traeger WHERE id = ?`, traegerID).Exec(ctx)
}

// CleanupDataByOGS removes all test data for a specific OGS.
// This should be called after tests to clean up created data.
func CleanupDataByOGS(tb testing.TB, db *bun.DB, ogsID string) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Delete in dependency order
	// Active domain
	_, _ = db.NewRaw(`DELETE FROM active.visits WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM active.attendance WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM active.group_supervisors WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM active.groups WHERE ogs_id = ?`, ogsID).Exec(ctx)

	// Activities domain
	_, _ = db.NewRaw(`DELETE FROM activities.student_enrollments WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM activities.groups WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM activities.categories WHERE ogs_id = ?`, ogsID).Exec(ctx)

	// Education domain
	_, _ = db.NewRaw(`DELETE FROM education.group_teacher WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM education.groups WHERE ogs_id = ?`, ogsID).Exec(ctx)

	// Users domain
	_, _ = db.NewRaw(`DELETE FROM users.teachers WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM users.students WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM users.staff WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM users.persons WHERE ogs_id = ?`, ogsID).Exec(ctx)

	// IoT domain
	_, _ = db.NewRaw(`DELETE FROM iot.devices WHERE ogs_id = ?`, ogsID).Exec(ctx)

	// Facilities domain
	_, _ = db.NewRaw(`DELETE FROM facilities.rooms WHERE ogs_id = ?`, ogsID).Exec(ctx)
}
