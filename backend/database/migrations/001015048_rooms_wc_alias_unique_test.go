package migrations

import (
	"context"
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dropWCAliasIndex removes the partial unique index so a test can pre-seed
// duplicate alias rows before re-running the up migration. SetupTestDB has
// already run every migration including 1.15.48, so the index exists at the
// start of every test in this file. The cleanup branch puts it back.
func dropWCAliasIndex(t *testing.T, db *testpkg.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx,
		fmt.Sprintf(`DROP INDEX IF EXISTS facilities.%s;`, roomsWCAliasUniqueIndex))
	require.NoError(t, err, "drop wc alias index")
}

// insertWCAliasRoom inserts a row directly via SQL so the test can stage a
// pre-migration state that the service layer would now reject. Returns the
// inserted row id.
func insertWCAliasRoom(t *testing.T, db *testpkg.DB, tenantID int64, name string) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var id int64
	err := db.NewRaw(`
		INSERT INTO facilities.rooms (tenant_id, name, building, floor, capacity, category)
		VALUES (?, ?, 'Test Building', 0, 10, 'Other')
		RETURNING id;
	`, tenantID, name).Scan(ctx, &id)
	require.NoError(t, err, "insert wc alias room %q", name)
	return id
}

// attachActiveGroup creates a minimal active.groups row referencing the room
// so the tie-breaker's "most active.groups attached" branch can be exercised.
// group_id is left NULL (allowed since migration 1.15.37) to avoid pulling in
// the activities.groups fixture chain — the migration's tie-breaker only
// counts rows. Returns the active group id (only useful for cleanup, which
// happens via cleanupWCAliasMigrationTest).
func attachActiveGroup(t *testing.T, db *testpkg.DB, tenantID, roomID int64) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var id int64
	err := db.NewRaw(`
		INSERT INTO active.groups (tenant_id, room_id, start_time)
		VALUES (?, ?, NOW())
		RETURNING id;
	`, tenantID, roomID).Scan(ctx, &id)
	require.NoError(t, err, "attach active group")
	return id
}

func countAliasRows(t *testing.T, db *testpkg.DB, tenantID int64) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var n int
	err := db.NewRaw(`
		SELECT COUNT(*) FROM facilities.rooms
		WHERE tenant_id = ? AND name IN ('WC', 'Toilette');
	`, tenantID).Scan(ctx, &n)
	require.NoError(t, err)
	return n
}

func roomNameByID(t *testing.T, db *testpkg.DB, id int64) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var name string
	err := db.NewRaw(`SELECT name FROM facilities.rooms WHERE id = ?;`, id).Scan(ctx, &name)
	require.NoError(t, err)
	return name
}

// TestRoomsWCAliasUniqueUp_NoDuplicates exercises the common production path:
// tenant has zero or one alias row, migration is a no-op for cleanup but
// still rebuilds the index.
func TestRoomsWCAliasUniqueUp_NoDuplicates(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)

	dropWCAliasIndex(t, db)
	wcID := insertWCAliasRoom(t, db, tenantID, "WC")

	require.NoError(t, roomsWCAliasUniqueUp(context.Background(), db))

	assert.Equal(t, "WC", roomNameByID(t, db, wcID))
	assert.Equal(t, 1, countAliasRows(t, db, tenantID))
}

// TestRoomsWCAliasUniqueUp_PreservesActiveUsage covers tie-breaker step 1:
// the room with active.groups attached wins, even when it's the non-canonical
// "Toilette" name. This is the scenario where a school manually created
// "Toilette" first, used it, then somehow ended up with a "WC" duplicate.
func TestRoomsWCAliasUniqueUp_PreservesActiveUsage(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)

	dropWCAliasIndex(t, db)
	wcID := insertWCAliasRoom(t, db, tenantID, "WC")
	toiletteID := insertWCAliasRoom(t, db, tenantID, "Toilette")
	// Toilette has a real active group attached → it should win the tie-breaker.
	_ = attachActiveGroup(t, db, tenantID, toiletteID)

	require.NoError(t, roomsWCAliasUniqueUp(context.Background(), db))

	assert.Equal(t, "Toilette", roomNameByID(t, db, toiletteID), "active-usage row should keep its alias name")
	assert.Contains(t, roomNameByID(t, db, wcID), "(archiviert v1.15.48", "loser should be renamed out of the alias namespace")
	assert.Equal(t, 1, countAliasRows(t, db, tenantID), "exactly one alias row must remain per tenant")

	// Backup row must capture the original name so a manual restore is possible.
	var backupOriginal, backupArchived string
	err := db.NewRaw(`
		SELECT original_name, archived_name FROM audit.wc_alias_migration_backup
		WHERE tenant_id = ? AND room_id = ?;
	`, tenantID, wcID).Scan(context.Background(), &backupOriginal, &backupArchived)
	require.NoError(t, err)
	assert.Equal(t, "WC", backupOriginal)
	assert.Equal(t, roomNameByID(t, db, wcID), backupArchived)
}

// TestRoomsWCAliasUniqueUp_PrefersCanonicalWC covers tie-breaker step 2:
// when active-group counts tie, "WC" wins over "Toilette".
func TestRoomsWCAliasUniqueUp_PrefersCanonicalWC(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)

	dropWCAliasIndex(t, db)
	wcID := insertWCAliasRoom(t, db, tenantID, "WC")
	toiletteID := insertWCAliasRoom(t, db, tenantID, "Toilette")

	require.NoError(t, roomsWCAliasUniqueUp(context.Background(), db))

	assert.Equal(t, "WC", roomNameByID(t, db, wcID), "canonical WC should win when group counts tie")
	assert.Contains(t, roomNameByID(t, db, toiletteID), "(archiviert v1.15.48", "Toilette must be archived when WC also exists")
	assert.Equal(t, 1, countAliasRows(t, db, tenantID))
}

// TestRoomsWCAliasUniqueUp_Idempotent ensures a second run after the first
// completed successfully is a no-op — important because the deploy framework
// can re-invoke a migration after a partial failure on the next run.
func TestRoomsWCAliasUniqueUp_Idempotent(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)

	dropWCAliasIndex(t, db)
	insertWCAliasRoom(t, db, tenantID, "WC")
	insertWCAliasRoom(t, db, tenantID, "Toilette")

	require.NoError(t, roomsWCAliasUniqueUp(context.Background(), db))
	firstRunCount := countAliasRows(t, db, tenantID)

	// Second run: archived rows no longer match the alias filter, so the
	// CTE finds nothing to do. The index is already in place.
	require.NoError(t, roomsWCAliasUniqueUp(context.Background(), db))
	assert.Equal(t, firstRunCount, countAliasRows(t, db, tenantID), "second run must not change alias count")

	var backupCount int
	err := db.NewRaw(`SELECT COUNT(*) FROM audit.wc_alias_migration_backup WHERE tenant_id = ?;`, tenantID).
		Scan(context.Background(), &backupCount)
	require.NoError(t, err)
	assert.Equal(t, 1, backupCount, "second run must not write duplicate backup rows")
}

// TestRoomsWCAliasUniqueUp_IndexBlocksFutureDuplicates is the whole point of
// this migration: after it runs, the database refuses a second alias even
// when the application-level guard is bypassed (e.g. via the TOCTOU race
// between two concurrent CreateRoom calls).
func TestRoomsWCAliasUniqueUp_IndexBlocksFutureDuplicates(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)

	// Index already exists from SetupTestDB's prior migration run.
	insertWCAliasRoom(t, db, tenantID, "WC")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, `
		INSERT INTO facilities.rooms (tenant_id, name, building, floor, capacity, category)
		VALUES (?, 'Toilette', 'Test Building', 0, 10, 'Other');
	`, tenantID)
	require.Error(t, err, "second alias insert must violate the partial unique index")
	assert.Contains(t, err.Error(), roomsWCAliasUniqueIndex,
		"violation must reference the alias index by name (not the generic name unique constraint)")
}

// TestRoomsWCAliasUniqueUp_TenantIsolation guards against the index being
// cross-tenant: every tenant must be allowed its own alias.
func TestRoomsWCAliasUniqueUp_TenantIsolation(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.OwnTenantRows(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)
	testpkg.OwnTenantRows(t, db, tenantB)

	insertWCAliasRoom(t, db, tenantA, "WC")
	insertWCAliasRoom(t, db, tenantB, "WC")

	assert.Equal(t, 1, countAliasRows(t, db, tenantA))
	assert.Equal(t, 1, countAliasRows(t, db, tenantB))
}
