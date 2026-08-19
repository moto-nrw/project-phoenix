package checkin_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// cleanupWCTestArtifacts removes WC infrastructure created by auto-create tests.
func cleanupWCTestArtifacts(t *testing.T, tc *svcTestContext) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stmts := []string{
		fmt.Sprintf(`DELETE FROM active.attendance WHERE visit_id IN (SELECT v.id FROM active.visits v JOIN active.groups ag ON ag.id = v.active_group_id JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name = '%s')`, constants.WCRoomName),
		fmt.Sprintf(`DELETE FROM active.visits WHERE active_group_id IN (SELECT ag.id FROM active.groups ag JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name = '%s')`, constants.WCRoomName),
		fmt.Sprintf(`DELETE FROM active.group_supervisors WHERE group_id IN (SELECT ag.id FROM active.groups ag JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name = '%s')`, constants.WCRoomName),
		fmt.Sprintf(`DELETE FROM active.groups WHERE room_id IN (SELECT id FROM facilities.rooms WHERE name = '%s')`, constants.WCRoomName),
		fmt.Sprintf(`DELETE FROM activities.schedules WHERE group_id IN (SELECT id FROM activities.groups WHERE name = '%s')`, constants.WCActivityName),
		fmt.Sprintf(`DELETE FROM activities.student_enrollments WHERE group_id IN (SELECT id FROM activities.groups WHERE name = '%s')`, constants.WCActivityName),
		fmt.Sprintf(`DELETE FROM activities.groups WHERE name = '%s'`, constants.WCActivityName),
		fmt.Sprintf(`DELETE FROM activities.categories WHERE name = '%s'`, constants.WCCategoryName),
		fmt.Sprintf(`DELETE FROM facilities.rooms WHERE name = '%s'`, constants.WCRoomName),
	}
	for _, stmt := range stmts {
		_, _ = tc.db.ExecContext(ctx, stmt)
	}
}

// cleanupSchulhofTestArtifacts removes Schulhof infrastructure created by auto-create tests.
func cleanupSchulhofTestArtifacts(t *testing.T, tc *svcTestContext) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stmts := []string{
		fmt.Sprintf(`DELETE FROM active.attendance WHERE visit_id IN (SELECT v.id FROM active.visits v JOIN active.groups ag ON ag.id = v.active_group_id JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name = '%s')`, constants.SchulhofRoomName),
		fmt.Sprintf(`DELETE FROM active.visits WHERE active_group_id IN (SELECT ag.id FROM active.groups ag JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name = '%s')`, constants.SchulhofRoomName),
		fmt.Sprintf(`DELETE FROM active.group_supervisors WHERE group_id IN (SELECT ag.id FROM active.groups ag JOIN facilities.rooms r ON r.id = ag.room_id WHERE r.name = '%s')`, constants.SchulhofRoomName),
		fmt.Sprintf(`DELETE FROM active.groups WHERE room_id IN (SELECT id FROM facilities.rooms WHERE name = '%s')`, constants.SchulhofRoomName),
		fmt.Sprintf(`DELETE FROM activities.schedules WHERE group_id IN (SELECT id FROM activities.groups WHERE name = '%s')`, constants.SchulhofActivityName),
		fmt.Sprintf(`DELETE FROM activities.student_enrollments WHERE group_id IN (SELECT id FROM activities.groups WHERE name = '%s')`, constants.SchulhofActivityName),
		fmt.Sprintf(`DELETE FROM activities.groups WHERE name = '%s'`, constants.SchulhofActivityName),
		fmt.Sprintf(`DELETE FROM activities.categories WHERE name = '%s'`, constants.SchulhofCategoryName),
		fmt.Sprintf(`DELETE FROM facilities.rooms WHERE name = '%s'`, constants.SchulhofRoomName),
	}
	for _, stmt := range stmts {
		_, _ = tc.db.ExecContext(ctx, stmt)
	}
}

// =============================================================================
// WC auto-create paths
// =============================================================================

func TestEnsureWCRoom_AutoCreatesWhenNotFound(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupWCTestArtifacts(t, tc)
	defer cleanupWCTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)
	room, err := tc.svc.EnsureWCRoomForTest(ctx)

	require.NoError(t, err, "ensureWCRoom should not return error")
	require.NotNil(t, room, "ensureWCRoom should return a room")
	assert.Equal(t, constants.WCRoomName, room.Name)
	assert.NotZero(t, room.ID, "Room should have a valid ID after creation")
}

func TestEnsureWCRoom_FindsExistingRoom(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupWCTestArtifacts(t, tc)
	defer cleanupWCTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)

	room1, err := tc.svc.EnsureWCRoomForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, room1)

	room2, err := tc.svc.EnsureWCRoomForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, room2)

	assert.Equal(t, room1.ID, room2.ID, "Should return same room, not create duplicate")
}

func TestEnsureWCCategory_AutoCreatesWhenNotFound(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupWCTestArtifacts(t, tc)
	defer cleanupWCTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)
	category, err := tc.svc.EnsureWCCategoryForTest(ctx)

	require.NoError(t, err, "ensureWCCategory should not return error")
	require.NotNil(t, category, "ensureWCCategory should return a category")
	assert.Equal(t, constants.WCCategoryName, category.Name)
	assert.NotZero(t, category.ID)
}

func TestEnsureWCCategory_FindsExistingCategory(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupWCTestArtifacts(t, tc)
	defer cleanupWCTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)

	cat1, err := tc.svc.EnsureWCCategoryForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, cat1)

	cat2, err := tc.svc.EnsureWCCategoryForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, cat2)

	assert.Equal(t, cat1.ID, cat2.ID, "Should return same category, not create duplicate")
}

func TestWcActivityGroup_FullAutoCreate(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupWCTestArtifacts(t, tc)
	defer cleanupWCTestArtifacts(t, tc)

	// Create staff for the created_by FK constraint
	db := testpkg.SetupTestDB(t)

	staff := testpkg.CreateTestStaff(t, db, "WCInternal", "Staff")

	ctx := tenant.WithTenantID(context.WithValue(context.Background(), device.CtxStaff, staff), 1)

	group, err := tc.svc.WCActivityGroupForTest(ctx)

	require.NoError(t, err, "wcActivityGroup should not return error")
	require.NotNil(t, group, "wcActivityGroup should return an activity group")
	assert.Equal(t, constants.WCActivityName, group.Name)
	assert.NotZero(t, group.ID)
}

func TestWcActivityGroup_FindsExisting(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupWCTestArtifacts(t, tc)
	defer cleanupWCTestArtifacts(t, tc)

	db := testpkg.SetupTestDB(t)

	staff := testpkg.CreateTestStaff(t, db, "WCExist", "Staff")

	ctx := tenant.WithTenantID(context.WithValue(context.Background(), device.CtxStaff, staff), 1)

	group1, err := tc.svc.WCActivityGroupForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, group1)

	group2, err := tc.svc.WCActivityGroupForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, group2)

	assert.Equal(t, group1.ID, group2.ID, "Should return same activity group, not create duplicate")
}

// =============================================================================
// Schulhof auto-create paths
// =============================================================================

func TestEnsureSchulhofRoom_AutoCreatesWhenNotFound(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupSchulhofTestArtifacts(t, tc)
	defer cleanupSchulhofTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)
	room, err := tc.svc.EnsureSchulhofRoomForTest(ctx)

	require.NoError(t, err, "ensureSchulhofRoom should not return error")
	require.NotNil(t, room, "ensureSchulhofRoom should return a room")
	assert.Equal(t, constants.SchulhofRoomName, room.Name)
	assert.NotZero(t, room.ID)
}

func TestEnsureSchulhofRoom_FindsExistingRoom(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupSchulhofTestArtifacts(t, tc)
	defer cleanupSchulhofTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)

	room1, err := tc.svc.EnsureSchulhofRoomForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, room1)

	room2, err := tc.svc.EnsureSchulhofRoomForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, room2)

	assert.Equal(t, room1.ID, room2.ID, "Should return same room, not create duplicate")
}

func TestEnsureSchulhofCategory_AutoCreatesWhenNotFound(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupSchulhofTestArtifacts(t, tc)
	defer cleanupSchulhofTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)
	category, err := tc.svc.EnsureSchulhofCategoryForTest(ctx)

	require.NoError(t, err, "ensureSchulhofCategory should not return error")
	require.NotNil(t, category, "ensureSchulhofCategory should return a category")
	assert.Equal(t, constants.SchulhofCategoryName, category.Name)
	assert.NotZero(t, category.ID)
}

func TestEnsureSchulhofCategory_FindsExistingCategory(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupSchulhofTestArtifacts(t, tc)
	defer cleanupSchulhofTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)

	cat1, err := tc.svc.EnsureSchulhofCategoryForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, cat1)

	cat2, err := tc.svc.EnsureSchulhofCategoryForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, cat2)

	assert.Equal(t, cat1.ID, cat2.ID, "Should return same category, not create duplicate")
}

func TestSchulhofActivityGroup_FullAutoCreate(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupSchulhofTestArtifacts(t, tc)
	defer cleanupSchulhofTestArtifacts(t, tc)

	db := testpkg.SetupTestDB(t)

	staff := testpkg.CreateTestStaff(t, db, "SchulhofInt", "Staff")

	ctx := tenant.WithTenantID(context.WithValue(context.Background(), device.CtxStaff, staff), 1)

	group, err := tc.svc.SchulhofActivityGroupForTest(ctx)

	require.NoError(t, err, "schulhofActivityGroup should not return error")
	require.NotNil(t, group, "schulhofActivityGroup should return an activity group")
	assert.Equal(t, constants.SchulhofActivityName, group.Name)
	assert.NotZero(t, group.ID)
}

func TestSchulhofActivityGroup_FindsExisting(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupSchulhofTestArtifacts(t, tc)
	defer cleanupSchulhofTestArtifacts(t, tc)

	db := testpkg.SetupTestDB(t)

	staff := testpkg.CreateTestStaff(t, db, "SchulhofExist", "Staff")

	ctx := tenant.WithTenantID(context.WithValue(context.Background(), device.CtxStaff, staff), 1)

	group1, err := tc.svc.SchulhofActivityGroupForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, group1)

	group2, err := tc.svc.SchulhofActivityGroupForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, group2)

	assert.Equal(t, group1.ID, group2.ID, "Should return same activity group, not create duplicate")
}

// =============================================================================
// NO STAFF CONTEXT: system-created auto-create paths (created_by = NULL)
// =============================================================================

func TestWcActivityGroup_NoStaffContext(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupWCTestArtifacts(t, tc)
	defer cleanupWCTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)

	group, err := tc.svc.WCActivityGroupForTest(ctx)

	require.NoError(t, err, "wcActivityGroup should succeed without staff context")
	require.NotNil(t, group, "wcActivityGroup should return an activity group")
	assert.Equal(t, constants.WCActivityName, group.Name)
	assert.NotZero(t, group.ID)
	assert.Nil(t, group.CreatedBy, "system-created WC group should have NULL created_by")
}

func TestSchulhofActivityGroup_NoStaffContext(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupSchulhofTestArtifacts(t, tc)
	defer cleanupSchulhofTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)

	group, err := tc.svc.SchulhofActivityGroupForTest(ctx)

	require.NoError(t, err, "schulhofActivityGroup should succeed without staff context")
	require.NotNil(t, group, "schulhofActivityGroup should return an activity group")
	assert.Equal(t, constants.SchulhofActivityName, group.Name)
	assert.NotZero(t, group.ID)
	assert.Nil(t, group.CreatedBy, "system-created Schulhof group should have NULL created_by")
}

// =============================================================================
// IS_SYSTEM FLAG: auto-provisioned infrastructure is flagged (issue #923)
// =============================================================================

func TestWcProvisioning_SetsIsSystemFlag(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupWCTestArtifacts(t, tc)
	defer cleanupWCTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)

	room, err := tc.svc.EnsureWCRoomForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, room)
	assert.True(t, room.IsSystem, "auto-created WC room must be flagged is_system")

	group, err := tc.svc.WCActivityGroupForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, group)
	assert.True(t, group.IsSystem, "auto-created WC activity group must be flagged is_system")

	category, err := tc.svc.EnsureWCCategoryForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, category)
	assert.True(t, category.IsSystem, "auto-created WC category must be flagged is_system")
}

func TestSchulhofProvisioning_SetsIsSystemFlag(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupSchulhofTestArtifacts(t, tc)
	defer cleanupSchulhofTestArtifacts(t, tc)

	ctx := tenant.WithTenantID(context.Background(), 1)

	room, err := tc.svc.EnsureSchulhofRoomForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, room)
	assert.True(t, room.IsSystem, "auto-created Schulhof room must be flagged is_system")

	group, err := tc.svc.SchulhofActivityGroupForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, group)
	assert.True(t, group.IsSystem, "auto-created Schulhof activity group must be flagged is_system")

	category, err := tc.svc.EnsureSchulhofCategoryForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, category)
	assert.True(t, category.IsSystem, "auto-created Schulhof category must be flagged is_system")
}

// =============================================================================
// WC room-name alias handling
// =============================================================================

func cleanupWCRoomAliasTestArtifacts(t *testing.T, db *bun.DB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	aliasArgs := []interface{}{constants.WCRoomName, constants.WCRoomAliasName}
	type stmt struct {
		sql  string
		args []interface{}
	}
	stmts := []stmt{
		{
			`DELETE FROM active.attendance WHERE visit_id IN (SELECT v.id FROM active.visits v JOIN active.groups ag ON ag.id = v.active_group_id JOIN facilities.rooms r ON r.id = ag.room_id WHERE LOWER(r.name) IN (LOWER(?), LOWER(?)))`,
			aliasArgs,
		},
		{
			`DELETE FROM active.visits WHERE active_group_id IN (SELECT ag.id FROM active.groups ag JOIN facilities.rooms r ON r.id = ag.room_id WHERE LOWER(r.name) IN (LOWER(?), LOWER(?)))`,
			aliasArgs,
		},
		{
			`DELETE FROM active.group_supervisors WHERE group_id IN (SELECT ag.id FROM active.groups ag JOIN facilities.rooms r ON r.id = ag.room_id WHERE LOWER(r.name) IN (LOWER(?), LOWER(?)))`,
			aliasArgs,
		},
		{
			`DELETE FROM active.groups WHERE room_id IN (SELECT id FROM facilities.rooms WHERE LOWER(name) IN (LOWER(?), LOWER(?)))`,
			aliasArgs,
		},
		{
			`DELETE FROM activities.schedules WHERE activity_group_id IN (SELECT id FROM activities.groups WHERE name = ?)`,
			[]interface{}{constants.WCActivityName},
		},
		{
			`DELETE FROM activities.student_enrollments WHERE activity_group_id IN (SELECT id FROM activities.groups WHERE name = ?)`,
			[]interface{}{constants.WCActivityName},
		},
		{
			`DELETE FROM activities.groups WHERE name = ?`,
			[]interface{}{constants.WCActivityName},
		},
		{
			`DELETE FROM activities.categories WHERE name = ?`,
			[]interface{}{constants.WCCategoryName},
		},
		{
			`DELETE FROM facilities.rooms WHERE LOWER(name) IN (LOWER(?), LOWER(?))`,
			aliasArgs,
		},
	}

	for _, s := range stmts {
		_, _ = db.NewRaw(s.sql, s.args...).Exec(ctx)
	}
}

func createWCRoomAliasRoom(t *testing.T, db *bun.DB, name string) *facilities.Room {
	t.Helper()

	ctx := tenant.WithTenantID(context.Background(), 1)
	room := &facilities.Room{
		Name:     name,
		Building: "Test Building",
	}
	room.SetTenantID(1)

	err := db.NewInsert().
		Model(room).
		ModelTableExpr(`facilities.rooms`).
		Scan(ctx)
	require.NoError(t, err)

	return room
}

func TestEnsureWCRoom_ReusesExistingToiletteAlias(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	cleanupWCRoomAliasTestArtifacts(t, tc.db)
	defer cleanupWCRoomAliasTestArtifacts(t, tc.db)

	aliasRoom := createWCRoomAliasRoom(t, tc.db, constants.WCRoomAliasName)

	room, err := tc.svc.EnsureWCRoomForTest(tenant.WithTenantID(context.Background(), 1))

	require.NoError(t, err)
	require.NotNil(t, room)
	assert.Equal(t, aliasRoom.ID, room.ID)
	assert.Equal(t, constants.WCRoomAliasName, room.Name)
}

func TestEnsureWCRoom_IgnoresLowercaseWCRoom(t *testing.T) {
	// Contract per issue #1184 review: only exact-case "WC" and "Toilette"
	// are toilet system rooms. ensureWCRoom goes through
	// services/facilities.FindToiletRoom which re-filters via IsWCRoomName, so a
	// lowercase "wc" must not be silently adopted here either.
	tc := setupCheckinServiceTest(t)

	cleanupWCRoomAliasTestArtifacts(t, tc.db)
	defer cleanupWCRoomAliasTestArtifacts(t, tc.db)

	lowercaseWC := createWCRoomAliasRoom(t, tc.db, "wc")

	room, err := tc.svc.EnsureWCRoomForTest(tenant.WithTenantID(context.Background(), 1))

	require.Error(t, err, "ensureWCRoom must not silently reuse lowercase wc; the duplicate-name collision must surface as an error")
	assert.Nil(t, room)

	// Lowercase room is untouched.
	var nameAfter string
	err = tc.db.NewSelect().
		Table("facilities.rooms").
		Column("name").
		Where("id = ?", lowercaseWC.ID).
		Scan(tenant.WithTenantID(context.Background(), 1), &nameAfter)
	require.NoError(t, err)
	assert.Equal(t, "wc", nameAfter)
}
