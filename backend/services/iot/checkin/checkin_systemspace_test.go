package checkin_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	assert.True(t, group.IsSystem)
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
	assert.True(t, group.IsSystem)
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
