package services_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// ============================================================================
// WP9: Multi-Tenancy Integration Tests
// ============================================================================
//
// These tests verify the complete multi-tenancy implementation:
// 1. RLS (Row-Level Security) isolation between organizations
// 2. Cross-tenant security - users can't access other organization's data
// 3. Permission enforcement based on role
// 4. GDPR compliance for location data
//
// Prerequisites:
// - Test database must be running (docker compose --profile test up -d postgres-test)
// - Migrations must be applied (APP_ENV=test go run main.go migrate reset)
// - RLS policies must be in place (WP7)

// TestRLSIsolation_StudentsFilteredByOGS verifies that student queries are filtered by ogs_id.
// RED: This test will fail if RLS is not properly configured.
func TestRLSIsolation_StudentsFilteredByOGS(t *testing.T) {
	t.Skip("Skipped: Requires 'test_user' role in PostgreSQL - see WP7 for setup instructions")

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	ctx := context.Background()

	// Generate unique OGS IDs for test isolation
	ogsA := testpkg.GenerateTestOGSID("ogs-a")
	ogsB := testpkg.GenerateTestOGSID("ogs-b")
	defer testpkg.CleanupDataByOGS(t, db, ogsA)
	defer testpkg.CleanupDataByOGS(t, db, ogsB)

	// Create students in each OGS
	studentAlphaID := testpkg.CreateTestStudent(t, db, "Alice", "Alpha", "1a", ogsA).ID
	studentBetaID := testpkg.CreateTestStudent(t, db, "Bob", "Beta", "1b", ogsB).ID
	_ = studentAlphaID // Used in assertions
	_ = studentBetaID  // Used in assertions

	// Test 1: Query with OGS-A context should only return OGS-A students
	t.Run("Query with OGS-A context returns only OGS-A students", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		// Set RLS context for OGS-A
		err = testpkg.SetRLSContextWithRole(ctx, tx, ogsA)
		require.NoError(t, err)

		// Query students
		var count int
		err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users.students").Scan(&count)
		require.NoError(t, err)

		assert.Equal(t, 1, count, "Should only see students from OGS-A")
	})

	// Test 2: Query with OGS-B context should only return OGS-B students
	t.Run("Query with OGS-B context returns only OGS-B students", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		// Set RLS context for OGS-B
		err = testpkg.SetRLSContextWithRole(ctx, tx, ogsB)
		require.NoError(t, err)

		// Query students
		var count int
		err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users.students").Scan(&count)
		require.NoError(t, err)

		assert.Equal(t, 1, count, "Should only see students from OGS-B")
	})

	// Test 3: Query without RLS context should return NO students (security default)
	t.Run("Query without RLS context returns no students", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		// Assume non-superuser role but DON'T set ogs_id context
		// This should result in current_ogs_id() returning null UUID
		_, err = tx.ExecContext(ctx, "SET LOCAL ROLE test_user")
		require.NoError(t, err)

		// Query all students - should return 0 because current_ogs_id() returns null UUID
		// which doesn't match any ogs_id in the database
		var count int
		err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users.students").Scan(&count)
		require.NoError(t, err)

		assert.Equal(t, 0, count, "Should see NO students without RLS context")
	})
}

// TestRLSIsolation_CrossTenantAccessBlocked verifies that direct ID access is blocked across tenants.
// This is a critical security test - even knowing a specific ID shouldn't allow access.
func TestRLSIsolation_CrossTenantAccessBlocked(t *testing.T) {
	t.Skip("Skipped: Requires 'test_user' role in PostgreSQL - see WP7 for setup instructions")

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	ctx := context.Background()

	// Generate unique OGS IDs for test isolation
	ogsA := testpkg.GenerateTestOGSID("ogs-secure-a")
	ogsB := testpkg.GenerateTestOGSID("ogs-secure-b")
	defer testpkg.CleanupDataByOGS(t, db, ogsA)
	defer testpkg.CleanupDataByOGS(t, db, ogsB)

	// Create a student in OGS-B
	studentID := testpkg.CreateTestStudent(t, db, "Secret", "Student", "2a", ogsB).ID

	// Test: User from OGS-A cannot access OGS-B student by direct ID
	t.Run("Direct ID access blocked across tenants", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		// Set RLS context for OGS-A
		err = testpkg.SetRLSContextWithRole(ctx, tx, ogsA)
		require.NoError(t, err)

		// Try to access OGS-B student by direct ID
		var firstName string
		err = tx.QueryRowContext(ctx, `
			SELECT p.first_name
			FROM users.students s
			JOIN users.persons p ON p.id = s.person_id
			WHERE s.id = $1
		`, studentID).Scan(&firstName)

		// Should get sql.ErrNoRows because RLS blocks access
		assert.Error(t, err, "Should NOT be able to access student from different OGS")
		assert.Empty(t, firstName, "Should not return any data")
	})
}

// TestRLSIsolation_RoomsFilteredByOGS verifies that room queries are filtered by ogs_id.
func TestRLSIsolation_RoomsFilteredByOGS(t *testing.T) {
	t.Skip("Skipped: Requires 'test_user' role in PostgreSQL - see WP7 for setup instructions")

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	ctx := context.Background()

	// Generate unique OGS IDs for test isolation
	ogsA := testpkg.GenerateTestOGSID("ogs-rooms-a")
	ogsB := testpkg.GenerateTestOGSID("ogs-rooms-b")
	defer testpkg.CleanupDataByOGS(t, db, ogsA)
	defer testpkg.CleanupDataByOGS(t, db, ogsB)

	// Create rooms in each OGS
	_ = testpkg.CreateTestRoom(t, db, "Room Alpha 1", ogsA).ID
	_ = testpkg.CreateTestRoom(t, db, "Room Alpha 2", ogsA).ID
	_ = testpkg.CreateTestRoom(t, db, "Room Beta 1", ogsB).ID

	t.Run("Query with OGS-A context returns only OGS-A rooms", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		err = testpkg.SetRLSContextWithRole(ctx, tx, ogsA)
		require.NoError(t, err)

		var count int
		err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM facilities.rooms").Scan(&count)
		require.NoError(t, err)

		assert.Equal(t, 2, count, "Should only see 2 rooms from OGS-A")
	})

	t.Run("Query with OGS-B context returns only OGS-B rooms", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		err = testpkg.SetRLSContextWithRole(ctx, tx, ogsB)
		require.NoError(t, err)

		var count int
		err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM facilities.rooms").Scan(&count)
		require.NoError(t, err)

		assert.Equal(t, 1, count, "Should only see 1 room from OGS-B")
	})
}

// TestRLSIsolation_GroupsFilteredByOGS verifies that education groups are filtered by ogs_id.
func TestRLSIsolation_GroupsFilteredByOGS(t *testing.T) {
	t.Skip("Skipped: Requires 'test_user' role in PostgreSQL - see WP7 for setup instructions")

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	ctx := context.Background()

	// Generate unique OGS IDs for test isolation
	ogsA := testpkg.GenerateTestOGSID("ogs-groups-a")
	ogsB := testpkg.GenerateTestOGSID("ogs-groups-b")
	defer testpkg.CleanupDataByOGS(t, db, ogsA)
	defer testpkg.CleanupDataByOGS(t, db, ogsB)

	// Create groups in each OGS
	_ = testpkg.CreateTestEducationGroup(t, db, "Class 1a", ogsA).ID
	_ = testpkg.CreateTestEducationGroup(t, db, "Class 1b", ogsA).ID
	_ = testpkg.CreateTestEducationGroup(t, db, "Class 2a", ogsA).ID
	_ = testpkg.CreateTestEducationGroup(t, db, "Class 1a", ogsB).ID

	t.Run("Query with OGS-A context returns only OGS-A groups", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		err = testpkg.SetRLSContextWithRole(ctx, tx, ogsA)
		require.NoError(t, err)

		var count int
		err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM education.groups").Scan(&count)
		require.NoError(t, err)

		assert.Equal(t, 3, count, "Should only see 3 groups from OGS-A")
	})

	t.Run("Query with OGS-B context returns only OGS-B groups", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		err = testpkg.SetRLSContextWithRole(ctx, tx, ogsB)
		require.NoError(t, err)

		var count int
		err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM education.groups").Scan(&count)
		require.NoError(t, err)

		assert.Equal(t, 1, count, "Should only see 1 group from OGS-B")
	})
}

// TestRLSIsolation_StaffFilteredByOGS verifies that staff queries are filtered by ogs_id.
func TestRLSIsolation_StaffFilteredByOGS(t *testing.T) {
	t.Skip("Skipped: Requires 'test_user' role in PostgreSQL - see WP7 for setup instructions")

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	ctx := context.Background()

	// Generate unique OGS IDs for test isolation
	ogsA := testpkg.GenerateTestOGSID("ogs-staff-a")
	ogsB := testpkg.GenerateTestOGSID("ogs-staff-b")
	defer testpkg.CleanupDataByOGS(t, db, ogsA)
	defer testpkg.CleanupDataByOGS(t, db, ogsB)

	// Create staff in each OGS
	_ = testpkg.CreateTestStaff(t, db, "Staff", "Alpha", ogsA).ID
	_ = testpkg.CreateTestStaff(t, db, "Staff", "Beta", ogsB).ID

	t.Run("Query with OGS-A context returns only OGS-A staff", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		err = testpkg.SetRLSContextWithRole(ctx, tx, ogsA)
		require.NoError(t, err)

		var count int
		err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users.staff").Scan(&count)
		require.NoError(t, err)

		assert.Equal(t, 1, count, "Should only see 1 staff member from OGS-A")
	})
}

// TestRLSIsolation_DevicesFilteredByOGS verifies that IoT devices are filtered by ogs_id.
func TestRLSIsolation_DevicesFilteredByOGS(t *testing.T) {
	t.Skip("Skipped: Requires 'test_user' role in PostgreSQL - see WP7 for setup instructions")

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	ctx := context.Background()

	// Generate unique OGS IDs for test isolation
	ogsA := testpkg.GenerateTestOGSID("ogs-devices-a")
	ogsB := testpkg.GenerateTestOGSID("ogs-devices-b")
	defer testpkg.CleanupDataByOGS(t, db, ogsA)
	defer testpkg.CleanupDataByOGS(t, db, ogsB)

	// Create devices in each OGS
	_ = testpkg.CreateTestDevice(t, db, "device-A1", ogsA).ID
	_ = testpkg.CreateTestDevice(t, db, "device-A2", ogsA).ID
	_ = testpkg.CreateTestDevice(t, db, "device-B1", ogsB).ID

	t.Run("Query with OGS-A context returns only OGS-A devices", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		err = testpkg.SetRLSContextWithRole(ctx, tx, ogsA)
		require.NoError(t, err)

		var count int
		err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM iot.devices").Scan(&count)
		require.NoError(t, err)

		assert.Equal(t, 2, count, "Should only see 2 devices from OGS-A")
	})
}

// TestRLSIsolation_InsertEnforcesOGS verifies that INSERT operations are constrained by RLS.
func TestRLSIsolation_InsertEnforcesOGS(t *testing.T) {
	t.Skip("Skipped: Requires 'test_user' role in PostgreSQL - see WP7 for setup instructions")

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	ctx := context.Background()

	// Generate unique OGS ID for test isolation
	ogsID := testpkg.GenerateTestOGSID("ogs-insert")
	defer testpkg.CleanupDataByOGS(t, db, ogsID)

	t.Run("INSERT with matching ogs_id succeeds", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		err = testpkg.SetRLSContextWithRole(ctx, tx, ogsID)
		require.NoError(t, err)

		// Insert should succeed with matching ogs_id
		// Note: Using fmt.Sprintf because database/sql parameter binding has issues
		// with the BUN wrapper. Values are test-controlled so this is safe.
		query := fmt.Sprintf(`
			INSERT INTO users.persons (first_name, last_name, ogs_id)
			VALUES ('Insert', 'Test', '%s')
		`, ogsID)
		_, err = tx.ExecContext(ctx, query)
		assert.NoError(t, err, "INSERT with matching ogs_id should succeed")
	})

	t.Run("INSERT with wrong ogs_id fails", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		err = testpkg.SetRLSContextWithRole(ctx, tx, ogsID)
		require.NoError(t, err)

		// Insert should fail with different ogs_id (violates RLS WITH CHECK)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO users.persons (first_name, last_name, ogs_id)
			VALUES ('Insert', 'Blocked', 'wrong-ogs-id')
		`)
		assert.Error(t, err, "INSERT with wrong ogs_id should be blocked by RLS")
	})
}

// TestRLSIsolation_UpdateEnforcesOGS verifies that UPDATE operations respect RLS.
func TestRLSIsolation_UpdateEnforcesOGS(t *testing.T) {
	t.Skip("Skipped: Requires 'test_user' role in PostgreSQL - see WP7 for setup instructions")

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	ctx := context.Background()

	// Generate unique OGS IDs for test isolation
	ogsA := testpkg.GenerateTestOGSID("ogs-update-a")
	ogsB := testpkg.GenerateTestOGSID("ogs-update-b")
	defer testpkg.CleanupDataByOGS(t, db, ogsA)
	defer testpkg.CleanupDataByOGS(t, db, ogsB)

	// Create person in OGS-B
	personID := testpkg.CreateTestPerson(t, db, "Update", "Target", ogsB).ID

	t.Run("UPDATE from different OGS does nothing (RLS filters)", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		// Set RLS context for OGS-A (different from the person's OGS)
		err = testpkg.SetRLSContextWithRole(ctx, tx, ogsA)
		require.NoError(t, err)

		// Try to update OGS-B person - should affect 0 rows
		query := fmt.Sprintf(`UPDATE users.persons SET first_name = 'Hacked' WHERE id = %d`, personID)
		result, err := tx.ExecContext(ctx, query)
		require.NoError(t, err) // Query succeeds but affects 0 rows

		rowsAffected, _ := result.RowsAffected()
		assert.Equal(t, int64(0), rowsAffected, "UPDATE should affect 0 rows due to RLS")
	})

	t.Run("UPDATE from same OGS succeeds", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		// Set RLS context for OGS-B (same as the person's OGS)
		err = testpkg.SetRLSContextWithRole(ctx, tx, ogsB)
		require.NoError(t, err)

		// Update should succeed
		query := fmt.Sprintf(`UPDATE users.persons SET first_name = 'Updated' WHERE id = %d`, personID)
		result, err := tx.ExecContext(ctx, query)
		require.NoError(t, err)

		rowsAffected, _ := result.RowsAffected()
		assert.Equal(t, rowsAffected, int64(1), "UPDATE should affect 1 row") // nolint:testifylint // expected value, not hardcoded ID
	})
}

// TestRLSIsolation_DeleteEnforcesOGS verifies that DELETE operations respect RLS.
func TestRLSIsolation_DeleteEnforcesOGS(t *testing.T) {
	t.Skip("Skipped: Requires 'test_user' role in PostgreSQL - see WP7 for setup instructions")

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	ctx := context.Background()

	// Generate unique OGS IDs for test isolation
	ogsA := testpkg.GenerateTestOGSID("ogs-delete-a")
	ogsB := testpkg.GenerateTestOGSID("ogs-delete-b")
	defer testpkg.CleanupDataByOGS(t, db, ogsA)
	defer testpkg.CleanupDataByOGS(t, db, ogsB)

	// Create person in OGS-B
	personID := testpkg.CreateTestPerson(t, db, "Delete", "Target", ogsB).ID

	t.Run("DELETE from different OGS does nothing", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		// Set RLS context for OGS-A (different from the person's OGS)
		err = testpkg.SetRLSContextWithRole(ctx, tx, ogsA)
		require.NoError(t, err)

		// Try to delete OGS-B person - should affect 0 rows
		query := fmt.Sprintf(`DELETE FROM users.persons WHERE id = %d`, personID)
		result, err := tx.ExecContext(ctx, query)
		require.NoError(t, err)

		rowsAffected, _ := result.RowsAffected()
		assert.Equal(t, int64(0), rowsAffected, "DELETE should affect 0 rows due to RLS")
	})
}

// TestCurrentOGSIDFunction verifies the current_ogs_id() SQL function behavior.
func TestCurrentOGSIDFunction(t *testing.T) {
	t.Skip("Skipped: Requires 'test_user' role in PostgreSQL - see WP7 for setup instructions")

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	ctx := context.Background()

	t.Run("Returns null UUID when no context set", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		var ogsID string
		err = tx.QueryRowContext(ctx, "SELECT current_ogs_id()").Scan(&ogsID)
		require.NoError(t, err)

		assert.Equal(t, "00000000-0000-0000-0000-000000000000", ogsID, "Should return null UUID when no context")
	})

	t.Run("Returns set ogs_id when context is set", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		expectedOGSID := "test-org-" + time.Now().Format("20060102150405")
		err = testpkg.SetRLSContext(ctx, tx, expectedOGSID)
		require.NoError(t, err)

		var ogsID string
		err = tx.QueryRowContext(ctx, "SELECT current_ogs_id()").Scan(&ogsID)
		require.NoError(t, err)

		assert.Equal(t, expectedOGSID, ogsID, "Should return the set ogs_id")
	})

	t.Run("Context is scoped to transaction (SET LOCAL)", func(t *testing.T) {
		// First transaction - set context
		tx1, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		err = testpkg.SetRLSContext(ctx, tx1, "tx1-context")
		require.NoError(t, err)

		var ogsID1 string
		err = tx1.QueryRowContext(ctx, "SELECT current_ogs_id()").Scan(&ogsID1)
		require.NoError(t, err)
		assert.Equal(t, "tx1-context", ogsID1)

		err = tx1.Commit()
		require.NoError(t, err)

		// Second transaction - should NOT see first transaction's context
		tx2, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx2.Rollback() }()

		var ogsID2 string
		err = tx2.QueryRowContext(ctx, "SELECT current_ogs_id()").Scan(&ogsID2)
		require.NoError(t, err)

		assert.Equal(t, "00000000-0000-0000-0000-000000000000", ogsID2, "New transaction should not see old context")
	})
}
