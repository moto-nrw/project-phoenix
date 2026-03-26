package test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tenant IDs used exclusively by purge_tenant tests.
// Values >= 42 satisfy the hermetic test scanner (int64(1)..int64(9) are flagged).
// Using 900+ range to avoid collisions with isolation tests (42-43) and other suites.
const (
	purgeTenantID         = int64(900)
	purgeRejectTenantID   = int64(901)
	purgeIsolationTenantA = int64(902)
	purgeIsolationTenantB = int64(903)
)

// purgeResult maps the JSONB summary returned by platform.purge_tenant().
type purgeResult struct {
	TenantID         int64         `json:"tenant_id"`
	TablesProcessed  int           `json:"tables_processed"`
	TotalRowsDeleted int64         `json:"total_rows_deleted"`
	Details          []purgeDetail `json:"details"`
}

type purgeDetail struct {
	Table       string `json:"table"`
	RowsDeleted int64  `json:"rows_deleted"`
}

// TestPurgeTenant_DeletesAllTenantData verifies that platform.purge_tenant()
// removes all tenant-scoped data across multiple schemas and returns an accurate
// JSONB summary. The school must be soft-deleted before purging.
func TestPurgeTenant_DeletesAllTenantData(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Arrange: create tenant infrastructure
	EnsureTestTenant(t, db, purgeTenantID)

	// Use a unique suffix to avoid collisions with stale data from failed runs
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// Insert tenant-scoped data into several representative tables.
	// We use raw SQL to directly target specific schemas with our tenant ID.

	// users.persons
	_, err := db.ExecContext(ctx, `
		INSERT INTO users.persons (tenant_id, first_name, last_name, created_at, updated_at)
		VALUES (?, ?, 'Person', NOW(), NOW())`, purgeTenantID, "Purge-"+suffix)
	require.NoError(t, err, "insert test person")

	// facilities.rooms
	_, err = db.ExecContext(ctx, `
		INSERT INTO facilities.rooms (tenant_id, name, building, created_at, updated_at)
		VALUES (?, ?, 'Test Building', NOW(), NOW())`, purgeTenantID, "Purge Room-"+suffix)
	require.NoError(t, err, "insert test room")

	// config.settings
	_, err = db.ExecContext(ctx, `
		INSERT INTO config.settings (tenant_id, key, value, category, created_at, updated_at)
		VALUES (?, ?, 'purge_test_value', 'general', NOW(), NOW())`, purgeTenantID, "purge_test_key_"+suffix)
	require.NoError(t, err, "insert test setting")

	// education.groups
	_, err = db.ExecContext(ctx, `
		INSERT INTO education.groups (tenant_id, name, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW())`, purgeTenantID, "Purge Group-"+suffix)
	require.NoError(t, err, "insert test education group")

	// Verify data exists before purge
	var personCount int
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM users.persons WHERE tenant_id = ?`, purgeTenantID).Scan(&personCount)
	require.NoError(t, err)
	require.Greater(t, personCount, 0, "persons should exist before purge")

	// Soft-delete the school (required precondition for purge)
	_, err = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, purgeTenantID)
	require.NoError(t, err, "soft-delete school")

	// Act: call the stored procedure (cast to BIGINT to match function signature)
	var resultJSON []byte
	err = db.QueryRowContext(ctx, `SELECT platform.purge_tenant(?::BIGINT)`, purgeTenantID).Scan(&resultJSON)
	require.NoError(t, err, "purge_tenant should succeed")

	// Parse JSONB result
	var result purgeResult
	err = json.Unmarshal(resultJSON, &result)
	require.NoError(t, err, "JSONB result should be valid JSON")

	// Assert summary
	assert.Equal(t, purgeTenantID, result.TenantID, "result should reference correct tenant")
	assert.Greater(t, result.TotalRowsDeleted, int64(0), "should have deleted rows")
	assert.Greater(t, result.TablesProcessed, 0, "should have processed tables")

	// Verify specific tables appear in the details
	detailTables := make(map[string]int64)
	for _, d := range result.Details {
		detailTables[d.Table] = d.RowsDeleted
	}
	assert.Contains(t, detailTables, "users.persons", "persons should appear in purge details")
	assert.Contains(t, detailTables, "facilities.rooms", "rooms should appear in purge details")
	assert.Contains(t, detailTables, "config.settings", "settings should appear in purge details")
	assert.Contains(t, detailTables, "education.groups", "groups should appear in purge details")
	assert.Contains(t, detailTables, "platform.schools", "school itself should appear in purge details")

	// Verify data is actually gone
	var count int

	err = db.QueryRowContext(ctx, `SELECT count(*) FROM users.persons WHERE tenant_id = ?`, purgeTenantID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "persons should be deleted")

	err = db.QueryRowContext(ctx, `SELECT count(*) FROM facilities.rooms WHERE tenant_id = ?`, purgeTenantID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "rooms should be deleted")

	err = db.QueryRowContext(ctx, `SELECT count(*) FROM config.settings WHERE tenant_id = ?`, purgeTenantID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "settings should be deleted")

	err = db.QueryRowContext(ctx, `SELECT count(*) FROM education.groups WHERE tenant_id = ?`, purgeTenantID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "education groups should be deleted")

	err = db.QueryRowContext(ctx, `SELECT count(*) FROM platform.schools WHERE id = ?`, purgeTenantID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "school record itself should be deleted")

	// Also verify the organization is NOT deleted (purge only removes school + tenant data)
	var orgCount int
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM platform.organizations WHERE id = ?`, purgeTenantID).Scan(&orgCount)
	require.NoError(t, err)
	assert.Equal(t, 1, orgCount, "organization should survive tenant purge")

	// Cleanup: remove the orphaned organization
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM platform.organizations WHERE id = ?`, purgeTenantID)
	})
}

// TestPurgeTenant_RejectsNonDeletedSchool verifies that purge_tenant() refuses
// to purge a school that has not been soft-deleted.
func TestPurgeTenant_RejectsNonDeletedSchool(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	EnsureTestTenant(t, db, purgeRejectTenantID)

	// Cleanup after test (school is NOT purged, so we must remove it manually)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM platform.schools WHERE id = ?`, purgeRejectTenantID)
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM platform.organizations WHERE id = ?`, purgeRejectTenantID)
	})

	// Act: attempt to purge without soft-deleting first
	var resultJSON []byte
	err := db.QueryRowContext(ctx, `SELECT platform.purge_tenant(?::BIGINT)`, purgeRejectTenantID).Scan(&resultJSON)

	// Assert: should fail with an error about soft-deletion
	require.Error(t, err, "purge_tenant should reject non-deleted school")
	assert.Contains(t, err.Error(), "does not exist or is not soft-deleted",
		"error message should explain the rejection reason")
}

// TestPurgeTenant_RejectsNonExistentSchool verifies that purge_tenant() raises
// an error when called with a school ID that does not exist.
func TestPurgeTenant_RejectsNonExistentSchool(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Act: call with a non-existent tenant ID
	nonExistentID := int64(99999)
	var resultJSON []byte
	err := db.QueryRowContext(ctx, `SELECT platform.purge_tenant(?::BIGINT)`, nonExistentID).Scan(&resultJSON)

	// Assert: should fail
	require.Error(t, err, "purge_tenant should reject non-existent school")
	assert.Contains(t, err.Error(), "does not exist or is not soft-deleted",
		"error message should explain the rejection reason")
}

// TestPurgeTenant_DoesNotAffectOtherTenants verifies that purging one tenant
// does not remove or modify data belonging to a different tenant.
func TestPurgeTenant_DoesNotAffectOtherTenants(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Arrange: create two tenants with data
	EnsureTestTenant(t, db, purgeIsolationTenantA)
	EnsureTestTenant(t, db, purgeIsolationTenantB)

	// Cleanup tenant B (the survivor) after the test.
	// Tenant A will be purged, so no cleanup needed for it beyond the org.
	t.Cleanup(func() {
		bgCtx := context.Background()
		// Clean tenant B data in FK-safe order
		_, _ = db.ExecContext(bgCtx, `DELETE FROM config.settings WHERE tenant_id = ?`, purgeIsolationTenantB)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM facilities.rooms WHERE tenant_id = ?`, purgeIsolationTenantB)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM users.persons WHERE tenant_id = ?`, purgeIsolationTenantB)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM platform.schools WHERE id = ?`, purgeIsolationTenantB)
		_, _ = db.ExecContext(bgCtx, `DELETE FROM platform.organizations WHERE id = ?`, purgeIsolationTenantB)
		// Cleanup orphaned org from purged tenant A
		_, _ = db.ExecContext(bgCtx, `DELETE FROM platform.organizations WHERE id = ?`, purgeIsolationTenantA)
	})

	// Use a unique suffix to avoid collisions with stale data from failed runs
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// Insert data for tenant A (will be purged)
	_, err := db.ExecContext(ctx, `
		INSERT INTO users.persons (tenant_id, first_name, last_name, created_at, updated_at)
		VALUES (?, ?, 'Person', NOW(), NOW())`, purgeIsolationTenantA, "TenantA-"+suffix)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO facilities.rooms (tenant_id, name, building, created_at, updated_at)
		VALUES (?, ?, 'Building A', NOW(), NOW())`, purgeIsolationTenantA, "TenantA Room-"+suffix)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO config.settings (tenant_id, key, value, category, created_at, updated_at)
		VALUES (?, ?, 'value_a', 'general', NOW(), NOW())`, purgeIsolationTenantA, "key_a_"+suffix)
	require.NoError(t, err)

	// Insert data for tenant B (must survive)
	_, err = db.ExecContext(ctx, `
		INSERT INTO users.persons (tenant_id, first_name, last_name, created_at, updated_at)
		VALUES (?, ?, 'Person', NOW(), NOW())`, purgeIsolationTenantB, "TenantB-"+suffix)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO facilities.rooms (tenant_id, name, building, created_at, updated_at)
		VALUES (?, ?, 'Building B', NOW(), NOW())`, purgeIsolationTenantB, "TenantB Room-"+suffix)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO config.settings (tenant_id, key, value, category, created_at, updated_at)
		VALUES (?, ?, 'value_b', 'general', NOW(), NOW())`, purgeIsolationTenantB, "key_b_"+suffix)
	require.NoError(t, err)

	// Record tenant B counts before purge
	var personsBefore, roomsBefore, settingsBefore int
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM users.persons WHERE tenant_id = ?`, purgeIsolationTenantB).Scan(&personsBefore)
	require.NoError(t, err)
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM facilities.rooms WHERE tenant_id = ?`, purgeIsolationTenantB).Scan(&roomsBefore)
	require.NoError(t, err)
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM config.settings WHERE tenant_id = ?`, purgeIsolationTenantB).Scan(&settingsBefore)
	require.NoError(t, err)

	// Soft-delete tenant A only
	_, err = db.ExecContext(ctx, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, purgeIsolationTenantA)
	require.NoError(t, err)

	// Act: purge tenant A
	var resultJSON []byte
	err = db.QueryRowContext(ctx, `SELECT platform.purge_tenant(?::BIGINT)`, purgeIsolationTenantA).Scan(&resultJSON)
	require.NoError(t, err, "purge_tenant should succeed for tenant A")

	// Assert: tenant A data is gone
	var count int
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM users.persons WHERE tenant_id = ?`, purgeIsolationTenantA).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "tenant A persons should be deleted")

	err = db.QueryRowContext(ctx, `SELECT count(*) FROM platform.schools WHERE id = ?`, purgeIsolationTenantA).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "tenant A school should be deleted")

	// Assert: tenant B data is untouched
	var personsAfter, roomsAfter, settingsAfter int
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM users.persons WHERE tenant_id = ?`, purgeIsolationTenantB).Scan(&personsAfter)
	require.NoError(t, err)
	assert.Equal(t, personsBefore, personsAfter, "tenant B persons count must not change")

	err = db.QueryRowContext(ctx, `SELECT count(*) FROM facilities.rooms WHERE tenant_id = ?`, purgeIsolationTenantB).Scan(&roomsAfter)
	require.NoError(t, err)
	assert.Equal(t, roomsBefore, roomsAfter, "tenant B rooms count must not change")

	err = db.QueryRowContext(ctx, `SELECT count(*) FROM config.settings WHERE tenant_id = ?`, purgeIsolationTenantB).Scan(&settingsAfter)
	require.NoError(t, err)
	assert.Equal(t, settingsBefore, settingsAfter, "tenant B settings count must not change")

	// Verify tenant B school still exists
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM platform.schools WHERE id = ?`, purgeIsolationTenantB).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "tenant B school must survive purge of tenant A")
}
