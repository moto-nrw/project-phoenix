package checkin_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// =============================================================================
// WC auto-create paths
// =============================================================================

func TestEnsureWCRoom_AutoCreatesWhenNotFound(t *testing.T) {
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	ctx := testpkg.Ctx(t)
	room, err := tc.svc.EnsureWCRoomForTest(ctx)

	require.NoError(t, err, "ensureWCRoom should not return error")
	require.NotNil(t, room, "ensureWCRoom should return a room")
	assert.Equal(t, constants.WCRoomName, room.Name)
	assert.NotZero(t, room.ID, "Room should have a valid ID after creation")
}

func TestEnsureWCRoom_FindsExistingRoom(t *testing.T) {
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	ctx := testpkg.Ctx(t)

	room1, err := tc.svc.EnsureWCRoomForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, room1)

	room2, err := tc.svc.EnsureWCRoomForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, room2)

	assert.Equal(t, room1.ID, room2.ID, "Should return same room, not create duplicate")
}

func TestEnsureWCCategory_AutoCreatesWhenNotFound(t *testing.T) {
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	ctx := testpkg.Ctx(t)
	category, err := tc.svc.EnsureWCCategoryForTest(ctx)

	require.NoError(t, err, "ensureWCCategory should not return error")
	require.NotNil(t, category, "ensureWCCategory should return a category")
	assert.Equal(t, constants.WCCategoryName, category.Name)
	assert.NotZero(t, category.ID)
}

func TestEnsureWCCategory_FindsExistingCategory(t *testing.T) {
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	ctx := testpkg.Ctx(t)

	cat1, err := tc.svc.EnsureWCCategoryForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, cat1)

	cat2, err := tc.svc.EnsureWCCategoryForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, cat2)

	assert.Equal(t, cat1.ID, cat2.ID, "Should return same category, not create duplicate")
}

func TestWcActivityGroup_FullAutoCreate(t *testing.T) {
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	// Create staff for the created_by FK constraint
	db := testpkg.SetupTestDB(t)

	staff := testpkg.CreateTestStaff(t, db, "WCInternal", "Staff")

	ctx := tenant.WithTenantID(context.WithValue(context.Background(), device.CtxStaff, staff), testpkg.Tenant(t))

	group, err := tc.svc.WCActivityGroupForTest(ctx)

	require.NoError(t, err, "wcActivityGroup should not return error")
	require.NotNil(t, group, "wcActivityGroup should return an activity group")
	assert.Equal(t, constants.WCActivityName, group.Name)
	assert.NotZero(t, group.ID)
}

func TestWcActivityGroup_FindsExisting(t *testing.T) {
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	db := testpkg.SetupTestDB(t)

	staff := testpkg.CreateTestStaff(t, db, "WCExist", "Staff")

	ctx := tenant.WithTenantID(context.WithValue(context.Background(), device.CtxStaff, staff), testpkg.Tenant(t))

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
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	ctx := testpkg.Ctx(t)
	room, err := tc.svc.EnsureSchulhofRoomForTest(ctx)

	require.NoError(t, err, "ensureSchulhofRoom should not return error")
	require.NotNil(t, room, "ensureSchulhofRoom should return a room")
	assert.Equal(t, constants.SchulhofRoomName, room.Name)
	assert.NotZero(t, room.ID)
}

func TestEnsureSchulhofRoom_FindsExistingRoom(t *testing.T) {
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	ctx := testpkg.Ctx(t)

	room1, err := tc.svc.EnsureSchulhofRoomForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, room1)

	room2, err := tc.svc.EnsureSchulhofRoomForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, room2)

	assert.Equal(t, room1.ID, room2.ID, "Should return same room, not create duplicate")
}

func TestEnsureSchulhofCategory_AutoCreatesWhenNotFound(t *testing.T) {
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	ctx := testpkg.Ctx(t)
	category, err := tc.svc.EnsureSchulhofCategoryForTest(ctx)

	require.NoError(t, err, "ensureSchulhofCategory should not return error")
	require.NotNil(t, category, "ensureSchulhofCategory should return a category")
	assert.Equal(t, constants.SchulhofCategoryName, category.Name)
	assert.NotZero(t, category.ID)
}

func TestEnsureSchulhofCategory_FindsExistingCategory(t *testing.T) {
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	ctx := testpkg.Ctx(t)

	cat1, err := tc.svc.EnsureSchulhofCategoryForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, cat1)

	cat2, err := tc.svc.EnsureSchulhofCategoryForTest(ctx)
	require.NoError(t, err)
	require.NotNil(t, cat2)

	assert.Equal(t, cat1.ID, cat2.ID, "Should return same category, not create duplicate")
}

func TestSchulhofActivityGroup_FullAutoCreate(t *testing.T) {
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	db := testpkg.SetupTestDB(t)

	staff := testpkg.CreateTestStaff(t, db, "SchulhofInt", "Staff")

	ctx := tenant.WithTenantID(context.WithValue(context.Background(), device.CtxStaff, staff), testpkg.Tenant(t))

	group, err := tc.svc.SchulhofActivityGroupForTest(ctx)

	require.NoError(t, err, "schulhofActivityGroup should not return error")
	require.NotNil(t, group, "schulhofActivityGroup should return an activity group")
	assert.Equal(t, constants.SchulhofActivityName, group.Name)
	assert.NotZero(t, group.ID)
}

func TestSchulhofActivityGroup_FindsExisting(t *testing.T) {
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	db := testpkg.SetupTestDB(t)

	staff := testpkg.CreateTestStaff(t, db, "SchulhofExist", "Staff")

	ctx := tenant.WithTenantID(context.WithValue(context.Background(), device.CtxStaff, staff), testpkg.Tenant(t))

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
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	ctx := testpkg.Ctx(t)

	group, err := tc.svc.WCActivityGroupForTest(ctx)

	require.NoError(t, err, "wcActivityGroup should succeed without staff context")
	require.NotNil(t, group, "wcActivityGroup should return an activity group")
	assert.Equal(t, constants.WCActivityName, group.Name)
	assert.NotZero(t, group.ID)
	assert.Nil(t, group.CreatedBy, "system-created WC group should have NULL created_by")
}

func TestSchulhofActivityGroup_NoStaffContext(t *testing.T) {
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	ctx := testpkg.Ctx(t)

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

func createWCRoomAliasRoom(t *testing.T, db *bun.DB, name string) *facilities.Room {
	t.Helper()

	ctx := testpkg.Ctx(t)
	room := &facilities.Room{
		Name:     name,
		Building: "Test Building",
	}
	room.SetTenantID(testpkg.Tenant(t))

	err := db.NewInsert().
		Model(room).
		ModelTableExpr(`facilities.rooms`).
		Scan(ctx)
	require.NoError(t, err)

	return room
}

func TestEnsureWCRoom_ReusesExistingToiletteAlias(t *testing.T) {
	t.Parallel()

	tc := setupCheckinServiceTest(t)

	aliasRoom := createWCRoomAliasRoom(t, tc.db, constants.WCRoomAliasName)

	room, err := tc.svc.EnsureWCRoomForTest(testpkg.Ctx(t))

	require.NoError(t, err)
	require.NotNil(t, room)
	assert.Equal(t, aliasRoom.ID, room.ID)
	assert.Equal(t, constants.WCRoomAliasName, room.Name)
}

func TestEnsureWCRoom_IgnoresLowercaseWCRoom(t *testing.T) {
	t.Parallel()

	// Contract per issue #1184 review: only exact-case "WC" and "Toilette"
	// are toilet system rooms. ensureWCRoom goes through
	// services/facilities.FindToiletRoom which re-filters via IsWCRoomName, so a
	// lowercase "wc" must not be silently adopted here either.
	tc := setupCheckinServiceTest(t)

	lowercaseWC := createWCRoomAliasRoom(t, tc.db, "wc")

	room, err := tc.svc.EnsureWCRoomForTest(testpkg.Ctx(t))

	require.Error(t, err, "ensureWCRoom must not silently reuse lowercase wc; the duplicate-name collision must surface as an error")
	assert.Nil(t, room)

	// Lowercase room is untouched.
	var nameAfter string
	err = tc.db.NewSelect().
		Table("facilities.rooms").
		Column("name").
		Where("id = ?", lowercaseWC.ID).
		Scan(testpkg.Ctx(t), &nameAfter)
	require.NoError(t, err)
	assert.Equal(t, "wc", nameAfter)
}
