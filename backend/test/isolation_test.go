// Package test provides tenant isolation tests that verify cross-tenant data
// cannot leak between different schools (tenants) in the multi-tenant system.
//
// WP 3.19: These tests exercise the defense-in-depth WHERE tenant_id = ? filters
// applied by every tenant-scoped repository. Each test creates data for two
// tenants of its own, then verifies that List and FindByID respect tenant
// boundaries in both directions.
package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// isolationTenants gives one test its OWN pair of tenants. The pair used to
// be two fixed IDs shared by every test in this file, which made the tests
// mutually destructive the moment they ran in parallel: each one wipes "its"
// tenants when it finishes (#2419).
func isolationTenants(tb testing.TB, db *bun.DB) (tenantA, tenantB int64) {
	tb.Helper()
	tenantA = UniqueTestTenantID(tb)
	tenantB = UniqueTestTenantID(tb)
	EnsureTestTenant(tb, db, tenantA)
	EnsureTestTenant(tb, db, tenantB)
	return tenantA, tenantB
}

// ctxForTenant returns a background context with the given tenant ID set.
func ctxForTenant(tenantID int64) context.Context {
	return tenant.WithTenantID(WithPackageTenantRuntime(context.Background()), tenantID)
}

// ============================================================================
// Users Domain
// ============================================================================

func TestTenantIsolation_StudentVisibility(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantA, tenantB := isolationTenants(t, db)

	// Arrange: create one student per tenant
	sA := CreateTestStudentForTenant(t, db, tenantA, "TenantA", "Student", "1a")
	sB := CreateTestStudentForTenant(t, db, tenantB, "TenantB", "Student", "1a")

	repo := api.NewUnboundStudentRepository(NewDatabase(t, db))

	// --- Tenant A perspective ---
	ctx42 := ctxForTenant(tenantA)

	students, err := TenantQuery1(t, ctx42, db, repo.List, nil)
	require.NoError(t, err)

	for _, s := range students {
		assert.Equal(t, tenantA, s.TenantID,
			"cross-tenant leak: tenant B student visible to tenant A (List)")
	}

	_, err = TenantQuery1(t, ctx42, db, repo.FindByID, any(sB.ID))
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B student %d", sB.ID)

	// --- Tenant B perspective ---
	ctx43 := ctxForTenant(tenantB)

	students, err = TenantQuery1(t, ctx43, db, repo.List, nil)
	require.NoError(t, err)

	for _, s := range students {
		assert.Equal(t, tenantB, s.TenantID,
			"cross-tenant leak: tenant A student visible to tenant B (List)")
	}

	_, err = TenantQuery1(t, ctx43, db, repo.FindByID, any(sA.ID))
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant B must not see tenant A student %d", sA.ID)
}

// ============================================================================
// Education Domain
// ============================================================================

func TestTenantIsolation_EducationGroupVisibility(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantA, tenantB := isolationTenants(t, db)

	gA := CreateTestEducationGroupForTenant(t, db, tenantA, "GroupA")
	gB := CreateTestEducationGroupForTenant(t, db, tenantB, "GroupB")

	repo := api.NewEducationGroupRepository(NewDatabase(t, db))

	// --- Tenant A ---
	ctx42 := ctxForTenant(tenantA)

	groups, err := TenantQuery1(t, ctx42, db, repo.List, nil)
	require.NoError(t, err)

	for _, g := range groups {
		assert.Equal(t, tenantA, g.TenantID,
			"cross-tenant leak: tenant B group visible to tenant A (List)")
	}

	_, err = TenantQuery1(t, ctx42, db, repo.FindByID, any(gB.ID))
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B group %d", gB.ID)

	// --- Tenant B ---
	ctx43 := ctxForTenant(tenantB)

	groups, err = TenantQuery1(t, ctx43, db, repo.List, nil)
	require.NoError(t, err)

	for _, g := range groups {
		assert.Equal(t, tenantB, g.TenantID,
			"cross-tenant leak: tenant A group visible to tenant B (List)")
	}

	_, err = TenantQuery1(t, ctx43, db, repo.FindByID, any(gA.ID))
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant B must not see tenant A group %d", gA.ID)
}

// ============================================================================
// Facilities Domain
// ============================================================================

func TestTenantIsolation_RoomVisibility(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantA, tenantB := isolationTenants(t, db)

	rA := CreateTestRoomForTenant(t, db, tenantA, "RoomA")
	rB := CreateTestRoomForTenant(t, db, tenantB, "RoomB")

	repo, err := api.NewFacilitiesRoomRepository(NewDatabase(t, db))
	require.NoError(t, err)

	// --- Tenant A ---
	ctx42 := ctxForTenant(tenantA)

	rooms, err := TenantQuery1(t, ctx42, db, repo.List, nil)
	require.NoError(t, err)

	for _, r := range rooms {
		assert.Equal(t, tenantA, r.TenantID,
			"cross-tenant leak: tenant B room visible to tenant A (List)")
	}

	_, err = TenantQuery1(t, ctx42, db, repo.FindByID, any(rB.ID))
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B room %d", rB.ID)

	// --- Tenant B ---
	ctx43 := ctxForTenant(tenantB)

	rooms, err = TenantQuery1(t, ctx43, db, repo.List, nil)
	require.NoError(t, err)

	for _, r := range rooms {
		assert.Equal(t, tenantB, r.TenantID,
			"cross-tenant leak: tenant A room visible to tenant B (List)")
	}

	_, err = TenantQuery1(t, ctx43, db, repo.FindByID, any(rA.ID))
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant B must not see tenant A room %d", rA.ID)
}

// ============================================================================
// Schedule Domain
// ============================================================================

func TestTenantIsolation_TimeframeVisibility(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantA, tenantB := isolationTenants(t, db)

	tfA := CreateTestTimeframeForTenant(t, db, tenantA, "TimeframeA")
	tfB := CreateTestTimeframeForTenant(t, db, tenantB, "TimeframeB")

	repos := api.NewUnobservedTimetableRepositories(MustDatabase(db))
	repo := repos.Timeframe

	// --- Tenant A ---
	ctx42 := ctxForTenant(tenantA)

	timeframes, err := TenantQuery0(t, ctx42, db, repo.ListAll)
	require.NoError(t, err)

	for _, tf := range timeframes {
		assert.Equal(t, tenantA, tf.TenantID,
			"cross-tenant leak: tenant B timeframe visible to tenant A (List)")
	}

	_, err = TenantQuery1(t, ctx42, db, repo.FindByID, tfB.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B timeframe %d", tfB.ID)

	// --- Tenant B ---
	ctx43 := ctxForTenant(tenantB)

	timeframes, err = TenantQuery0(t, ctx43, db, repo.ListAll)
	require.NoError(t, err)

	for _, tf := range timeframes {
		assert.Equal(t, tenantB, tf.TenantID,
			"cross-tenant leak: tenant A timeframe visible to tenant B (List)")
	}

	_, err = TenantQuery1(t, ctx43, db, repo.FindByID, tfA.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant B must not see tenant A timeframe %d", tfA.ID)
}

// ============================================================================
// IoT Domain
// ============================================================================

func TestTenantIsolation_DeviceVisibility(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantA, tenantB := isolationTenants(t, db)

	dA := CreateTestDeviceForTenant(t, db, tenantA, "DEV-A")
	dB := CreateTestDeviceForTenant(t, db, tenantB, "DEV-B")

	// Device reads go through the Device Fleet owner (#2676), which resolves
	// room names through the Facilities owner.
	repo, err := api.NewDeviceRepository(NewDatabase(t, db))
	require.NoError(t, err)

	// --- Tenant A ---
	ctx42 := ctxForTenant(tenantA)

	devices, err := TenantQuery1(t, ctx42, db, repo.List, nil)
	require.NoError(t, err)

	for _, d := range devices {
		assert.Equal(t, tenantA, d.TenantID,
			"cross-tenant leak: tenant B device visible to tenant A (List)")
	}

	_, err = TenantQuery1(t, ctx42, db, repo.FindByID, any(dB.ID))
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B device %d", dB.ID)

	// --- Tenant B ---
	ctx43 := ctxForTenant(tenantB)

	devices, err = TenantQuery1(t, ctx43, db, repo.List, nil)
	require.NoError(t, err)

	for _, d := range devices {
		assert.Equal(t, tenantB, d.TenantID,
			"cross-tenant leak: tenant A device visible to tenant B (List)")
	}

	_, err = TenantQuery1(t, ctx43, db, repo.FindByID, any(dA.ID))
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant B must not see tenant A device %d", dA.ID)
}

// TestTenantIsolation_DisplayVisibility covers display.displays, the Device
// Fleet owner's second table (#2676). Every id-addressed display read and
// write carries a tenant predicate; only the dashboard token lookup runs
// without one, and only inside the admin scope.
func TestTenantIsolation_DisplayVisibility(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantA, tenantB := isolationTenants(t, db)

	fleet, err := owners.NewDeviceFleet(NewDatabase(t, db))
	require.NoError(t, err)

	ctxA := ctxForTenant(tenantA)
	ctxB := ctxForTenant(tenantB)

	displayA, _, err := fleet.CreateDisplay(ctxA, "Eingang A")
	require.NoError(t, err)
	displayB, _, err := fleet.CreateDisplay(ctxB, "Eingang B")
	require.NoError(t, err)

	// --- Tenant A ---
	listed, err := fleet.ListDisplays(ctxA)
	require.NoError(t, err)
	for _, display := range listed {
		assert.Equal(t, tenantA, display.TenantID,
			"cross-tenant leak: tenant B display visible to tenant A (ListDisplays)")
	}

	name := "Umbenannt"
	_, err = fleet.UpdateDisplay(ctxA, displayB.ID, &name, nil)
	assert.Error(t, err,
		"cross-tenant UpdateDisplay should fail: tenant A must not write tenant B display %d", displayB.ID)

	_, err = fleet.RegenerateDisplayToken(ctxA, displayB.ID)
	assert.Error(t, err,
		"cross-tenant RegenerateDisplayToken should fail for display %d", displayB.ID)

	err = fleet.DeleteDisplay(ctxA, displayB.ID)
	assert.Error(t, err,
		"cross-tenant DeleteDisplay should fail for display %d", displayB.ID)

	// --- Tenant B ---
	listed, err = fleet.ListDisplays(ctxB)
	require.NoError(t, err)
	for _, display := range listed {
		assert.Equal(t, tenantB, display.TenantID,
			"cross-tenant leak: tenant A display visible to tenant B (ListDisplays)")
	}

	err = fleet.DeleteDisplay(ctxB, displayA.ID)
	assert.Error(t, err,
		"cross-tenant DeleteDisplay should fail for display %d", displayA.ID)

	// The owner's own rows stay reachable, so the assertions above prove
	// isolation rather than a broken composition.
	_, err = fleet.UpdateDisplay(ctxA, displayA.ID, &name, nil)
	require.NoError(t, err, "a display of the caller's own tenant must stay writable")
}

// ============================================================================
// Active Domain
// ============================================================================

func TestTenantIsolation_ActiveGroupVisibility(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantA, tenantB := isolationTenants(t, db)

	agA := CreateTestActiveGroupForTenant(t, db, tenantA)
	agB := CreateTestActiveGroupForTenant(t, db, tenantB)

	repo, _, _ := api.NewSessionCleanupRepositories(NewDatabase(t, db), owners.MustNewUnobservedTimetable(NewDatabase(t, db)))
	presence := PresenceModule(t, db)

	// --- Tenant A ---
	ctx42 := ctxForTenant(tenantA)

	groups, err := TenantQuery1(t, ctx42, db, presence.QueryLiveGroups, studentpresence.LiveGroupFilter{})
	require.NoError(t, err)

	for _, g := range groups {
		assert.Equal(t, tenantA, g.TenantID,
			"cross-tenant leak: tenant B active group visible to tenant A (List)")
	}

	_, err = TenantQuery1(t, ctx42, db, repo.FindSession, agB.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B active group %d", agB.ID)

	// --- Tenant B ---
	ctx43 := ctxForTenant(tenantB)

	groups, err = TenantQuery1(t, ctx43, db, presence.QueryLiveGroups, studentpresence.LiveGroupFilter{})
	require.NoError(t, err)

	for _, g := range groups {
		assert.Equal(t, tenantB, g.TenantID,
			"cross-tenant leak: tenant A active group visible to tenant B (List)")
	}

	_, err = TenantQuery1(t, ctx43, db, repo.FindSession, agA.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant B must not see tenant A active group %d", agA.ID)
}

// ============================================================================
// Activities Domain
// ============================================================================

func TestTenantIsolation_ActivityCategoryVisibility(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantA, tenantB := isolationTenants(t, db)

	catA := CreateTestActivityCategoryForTenant(t, db, tenantA, "CatA")
	catB := CreateTestActivityCategoryForTenant(t, db, tenantB, "CatB")

	repo := api.NewUnobservedTimetableRepositories(MustDatabase(db)).ActivityCategory

	// --- Tenant A ---
	ctx42 := ctxForTenant(tenantA)

	categories, err := TenantQuery0(t, ctx42, db, repo.ListAll)
	require.NoError(t, err)

	for _, c := range categories {
		assert.Equal(t, tenantA, c.TenantID,
			"cross-tenant leak: tenant B category visible to tenant A (List)")
	}

	_, err = TenantQuery1(t, ctx42, db, repo.FindByID, catB.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B category %d", catB.ID)

	// --- Tenant B ---
	ctx43 := ctxForTenant(tenantB)

	categories, err = TenantQuery0(t, ctx43, db, repo.ListAll)
	require.NoError(t, err)

	for _, c := range categories {
		assert.Equal(t, tenantB, c.TenantID,
			"cross-tenant leak: tenant A category visible to tenant B (List)")
	}

	_, err = TenantQuery1(t, ctx43, db, repo.FindByID, catA.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant B must not see tenant A category %d", catA.ID)
}

// ============================================================================
// Cross-Tenant Write Tests (RowsAffected guard)
// ============================================================================

// TestCrossTenantWrite_RowsAffectedGuard verifies that UPDATE and DELETE
// operations across tenant boundaries are blocked or silently ignored.
//
// The defense-in-depth strategy adds WHERE tenant_id = ? (from context) to
// every tenant-scoped write. When tenant A's context targets tenant B's row,
// the WHERE matches zero rows:
//   - Update / AssignToGroup: AssertRowsAffected(result, 1) returns an error
//   - Delete: no AssertRowsAffected call → silent no-op (0 rows deleted)
func TestCrossTenantWrite_RowsAffectedGuard(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantA, tenantB := isolationTenants(t, db)

	// Arrange: create one record per tenant in each domain
	roomA := CreateTestRoomForTenant(t, db, tenantA, "WriteRoomA")
	groupA := CreateTestEducationGroupForTenant(t, db, tenantA, "WriteGroupA")

	ctxA := ctxForTenant(tenantA)
	ctxB := ctxForTenant(tenantB)

	// ------------------------------------------------------------------
	// 1. base.Repository.Update across 2 domains (the student writes are
	//    pinned in the People Directory contract suite)
	// ------------------------------------------------------------------

	t.Run("room update blocked", func(t *testing.T) {
		repo, err := api.NewFacilitiesRoomRepository(NewDatabase(t, db))
		require.NoError(t, err)
		stored, err := TenantQuery1(t, ctxA, db, repo.FindByID, any(roomA.ID))
		require.NoError(t, err)
		err = TenantCommand1(t, ctxB, db, repo.Update, stored)
		require.Error(t, err, "cross-tenant room update must fail")
	})

	t.Run("education group update blocked", func(t *testing.T) {
		repo := api.NewEducationGroupRepository(NewDatabase(t, db))
		stored, err := TenantQuery1(t, ctxA, db, repo.FindByID, any(groupA.ID))
		require.NoError(t, err)
		err = TenantCommand1(t, ctxB, db, repo.Update, stored)
		require.Error(t, err, "cross-tenant group update must fail")
		assert.Contains(t, err.Error(), "rows affected",
			"error should mention rows affected guard")
	})

	// ------------------------------------------------------------------
	// 5. Delete: silent no-op (no AssertRowsAffected in base.Delete)
	// ------------------------------------------------------------------

	t.Run("room delete blocked", func(t *testing.T) {
		repo, err := api.NewFacilitiesRoomRepository(NewDatabase(t, db))
		require.NoError(t, err)

		// Attempt cross-tenant delete: ctxB tries to delete roomA (tenant A)
		err = TenantCommand1(t, ctxB, db, repo.Delete, any(roomA.ID))
		require.Error(t, err, "cross-tenant room delete must fail")

		// Verify the room still exists from tenant A's perspective
		found, err := TenantQuery1(t, ctxA, db, repo.FindByID, any(roomA.ID))
		require.NoError(t, err, "room should still be accessible to its own tenant")
		assert.Equal(t, roomA.ID, found.ID, "room must survive cross-tenant delete attempt")
	})
}
