package migrations

// Regression coverage for the 1.15.169 repair (issue #923 review finding):
// a 'Schulhof Freispiel' activity provisioned through ToggleSupervision ->
// EnsureInfrastructure(ctx, staffID) carries a staff created_by, so the
// 1.15.168 backfill (created_by IS NULL) skipped it and left it visible in
// staff-facing lists. The repair flags such rows via their link to the
// tenant's 'Schulhof' category, while leaving staff activities that merely
// share a system name untouched.

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertIsSystemTestCategory inserts a category directly via SQL so the test
// can stage a pre-repair state with an exact name — the fixture helpers
// uniquify names, which would defeat the migration's name-based predicate.
// Returns the inserted row id.
func insertIsSystemTestCategory(t *testing.T, db *testpkg.DB, tenantID int64, name string, isSystem bool) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var id int64
	err := db.NewRaw(`
		INSERT INTO activities.categories (tenant_id, name, is_system)
		VALUES (?, ?, ?)
		RETURNING id;
	`, tenantID, name, isSystem).Scan(ctx, &id)
	require.NoError(t, err, "insert category %q", name)
	return id
}

// insertIsSystemTestGroup inserts an activity group directly via SQL with an
// explicit is_system value. createdBy may be nil (system-created) or a staff
// id in the same tenant. Returns the row id.
func insertIsSystemTestGroup(t *testing.T, db *testpkg.DB, tenantID int64, name string, categoryID int64, createdBy *int64, isSystem bool) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var id int64
	err := db.NewRaw(`
		INSERT INTO activities.groups (tenant_id, name, max_participants, is_open, category_id, created_by, is_system)
		VALUES (?, ?, 20, TRUE, ?, ?, ?)
		RETURNING id;
	`, tenantID, name, categoryID, createdBy, isSystem).Scan(ctx, &id)
	require.NoError(t, err, "insert activity group %q", name)
	return id
}

func groupIsSystem(t *testing.T, db *testpkg.DB, groupID int64) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var isSystem bool
	err := db.NewRaw(`SELECT is_system FROM activities.groups WHERE id = ?;`, groupID).Scan(ctx, &isSystem)
	require.NoError(t, err, "load is_system for group %d", groupID)
	return isSystem
}

// TestRepairSchulhofIsSystemBackfill_FlagsStaffProvisionedRows stages the
// exact state 1.15.168 leaves behind on a deployed database and runs the
// repair. The load-bearing fixture is the staff-provisioned Schulhof
// activity: created_by set to a real staff id, linked to the tenant's
// 'Schulhof' category, still is_system = FALSE — the row the created_by IS
// NULL predicate skipped.
func TestRepairSchulhofIsSystemBackfill_FlagsStaffProvisionedRows(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Schulhof", "Supervisor")

	// 1.15.168 already flagged the category (name-based, unambiguous).
	schulhofCategory := insertIsSystemTestCategory(t, db, tenantID, "Schulhof", true)
	craftCategory := insertIsSystemTestCategory(t, db, tenantID, "Basteln", false)

	// The review blocker: ToggleSupervision -> EnsureInfrastructure(ctx, staffID)
	// provisioned the system activity with the supervising staff's id, so
	// 1.15.168 left it unflagged.
	staffProvisioned := insertIsSystemTestGroup(t, db, tenantID, "Schulhof Freispiel", schulhofCategory, &staff.ID, false)
	// Settings-toggle provisioning (created_by NULL): 1.15.168 already
	// flagged these; the repair must keep them flagged.
	alreadyFlagged := insertIsSystemTestGroup(t, db, tenantID, "Schulhof Freispiel", schulhofCategory, nil, true)
	// A staff member's own activity that merely shares the name must stay
	// visible in staff-facing lists.
	staffOwnSameName := insertIsSystemTestGroup(t, db, tenantID, "Schulhof Freispiel", craftCategory, &staff.ID, false)
	// A staff-owned activity named 'WC' is out of the repair's scope
	// entirely and must stay unflagged.
	staffOwnWC := insertIsSystemTestGroup(t, db, tenantID, "WC", craftCategory, &staff.ID, false)

	require.NoError(t, repairSchulhofIsSystemBackfillUp(ctx, db))

	assert.True(t, groupIsSystem(t, db, staffProvisioned),
		"staff-provisioned Schulhof activity (created_by set) must be flagged via its category link")
	assert.True(t, groupIsSystem(t, db, alreadyFlagged),
		"rows 1.15.168 already flagged must stay flagged")
	assert.False(t, groupIsSystem(t, db, staffOwnSameName),
		"staff activity sharing the name but not the Schulhof category must stay unflagged")
	assert.False(t, groupIsSystem(t, db, staffOwnWC),
		"staff-owned activity named 'WC' must stay unflagged")

	// Idempotency: a second run must not change any verdict.
	require.NoError(t, repairSchulhofIsSystemBackfillUp(ctx, db))
	assert.True(t, groupIsSystem(t, db, staffProvisioned))
	assert.False(t, groupIsSystem(t, db, staffOwnSameName))
	assert.False(t, groupIsSystem(t, db, staffOwnWC))
}
