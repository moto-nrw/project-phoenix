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

	repoActive "github.com/moto-nrw/project-phoenix/database/repositories/active"
	repoActivities "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	repoAudit "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	repoAuth "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	repoEducation "github.com/moto-nrw/project-phoenix/database/repositories/education"
	repoFacilities "github.com/moto-nrw/project-phoenix/database/repositories/facilities"
	repoFeedback "github.com/moto-nrw/project-phoenix/database/repositories/feedback"
	repoIot "github.com/moto-nrw/project-phoenix/database/repositories/iot"
	repoSchedule "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	repoUsers "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/models/users"
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
	return tenant.WithTenantID(context.Background(), tenantID)
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

	repo := repoUsers.NewStudentRepository(db)

	// --- Tenant A perspective ---
	ctx42 := ctxForTenant(tenantA)

	students, err := repo.List(ctx42, nil)
	require.NoError(t, err)

	for _, s := range students {
		assert.Equal(t, tenantA, s.TenantID,
			"cross-tenant leak: tenant B student visible to tenant A (List)")
	}

	_, err = repo.FindByID(ctx42, sB.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B student %d", sB.ID)

	// --- Tenant B perspective ---
	ctx43 := ctxForTenant(tenantB)

	students, err = repo.List(ctx43, nil)
	require.NoError(t, err)

	for _, s := range students {
		assert.Equal(t, tenantB, s.TenantID,
			"cross-tenant leak: tenant A student visible to tenant B (List)")
	}

	_, err = repo.FindByID(ctx43, sA.ID)
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

	repo := repoEducation.NewGroupRepository(db)

	// --- Tenant A ---
	ctx42 := ctxForTenant(tenantA)

	groups, err := repo.List(ctx42, nil)
	require.NoError(t, err)

	for _, g := range groups {
		assert.Equal(t, tenantA, g.TenantID,
			"cross-tenant leak: tenant B group visible to tenant A (List)")
	}

	_, err = repo.FindByID(ctx42, gB.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B group %d", gB.ID)

	// --- Tenant B ---
	ctx43 := ctxForTenant(tenantB)

	groups, err = repo.List(ctx43, nil)
	require.NoError(t, err)

	for _, g := range groups {
		assert.Equal(t, tenantB, g.TenantID,
			"cross-tenant leak: tenant A group visible to tenant B (List)")
	}

	_, err = repo.FindByID(ctx43, gA.ID)
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

	repo := repoFacilities.NewRoomRepository(db)

	// --- Tenant A ---
	ctx42 := ctxForTenant(tenantA)

	rooms, err := repo.List(ctx42, nil)
	require.NoError(t, err)

	for _, r := range rooms {
		assert.Equal(t, tenantA, r.TenantID,
			"cross-tenant leak: tenant B room visible to tenant A (List)")
	}

	_, err = repo.FindByID(ctx42, rB.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B room %d", rB.ID)

	// --- Tenant B ---
	ctx43 := ctxForTenant(tenantB)

	rooms, err = repo.List(ctx43, nil)
	require.NoError(t, err)

	for _, r := range rooms {
		assert.Equal(t, tenantB, r.TenantID,
			"cross-tenant leak: tenant A room visible to tenant B (List)")
	}

	_, err = repo.FindByID(ctx43, rA.ID)
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

	repo := repoSchedule.NewTimeframeRepository(db)

	// --- Tenant A ---
	ctx42 := ctxForTenant(tenantA)

	timeframes, err := repo.List(ctx42, nil)
	require.NoError(t, err)

	for _, tf := range timeframes {
		assert.Equal(t, tenantA, tf.TenantID,
			"cross-tenant leak: tenant B timeframe visible to tenant A (List)")
	}

	_, err = repo.FindByID(ctx42, tfB.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B timeframe %d", tfB.ID)

	// --- Tenant B ---
	ctx43 := ctxForTenant(tenantB)

	timeframes, err = repo.List(ctx43, nil)
	require.NoError(t, err)

	for _, tf := range timeframes {
		assert.Equal(t, tenantB, tf.TenantID,
			"cross-tenant leak: tenant A timeframe visible to tenant B (List)")
	}

	_, err = repo.FindByID(ctx43, tfA.ID)
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

	repo := repoIot.NewDeviceRepository(db)

	// --- Tenant A ---
	ctx42 := ctxForTenant(tenantA)

	devices, err := repo.List(ctx42, nil)
	require.NoError(t, err)

	for _, d := range devices {
		assert.Equal(t, tenantA, d.TenantID,
			"cross-tenant leak: tenant B device visible to tenant A (List)")
	}

	_, err = repo.FindByID(ctx42, dB.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B device %d", dB.ID)

	// --- Tenant B ---
	ctx43 := ctxForTenant(tenantB)

	devices, err = repo.List(ctx43, nil)
	require.NoError(t, err)

	for _, d := range devices {
		assert.Equal(t, tenantB, d.TenantID,
			"cross-tenant leak: tenant A device visible to tenant B (List)")
	}

	_, err = repo.FindByID(ctx43, dA.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant B must not see tenant A device %d", dA.ID)
}

// ============================================================================
// Auth Domain
// ============================================================================

func TestTenantIsolation_TokenVisibility(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantA, tenantB := isolationTenants(t, db)

	// Tokens require an account (accounts are not tenant-scoped).
	acctA := CreateTestAccount(t, db, "token-isolation-a")
	acctB := CreateTestAccount(t, db, "token-isolation-b")

	tkA := CreateTestTokenForTenant(t, db, tenantA, acctA.ID)
	tkB := CreateTestTokenForTenant(t, db, tenantB, acctB.ID)

	repo := repoAuth.NewTokenRepository(db)

	// --- Tenant A ---
	ctx42 := ctxForTenant(tenantA)

	tokens, err := repo.List(ctx42, nil)
	require.NoError(t, err)

	for _, tk := range tokens {
		assert.Equal(t, tenantA, tk.TenantID,
			"cross-tenant leak: tenant B token visible to tenant A (List)")
	}

	_, err = repo.FindByID(ctx42, tkB.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B token %d", tkB.ID)

	// --- Tenant B ---
	ctx43 := ctxForTenant(tenantB)

	tokens, err = repo.List(ctx43, nil)
	require.NoError(t, err)

	for _, tk := range tokens {
		assert.Equal(t, tenantB, tk.TenantID,
			"cross-tenant leak: tenant A token visible to tenant B (List)")
	}

	_, err = repo.FindByID(ctx43, tkA.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant B must not see tenant A token %d", tkA.ID)
}

// ============================================================================
// Feedback Domain
// ============================================================================

func TestTenantIsolation_FeedbackEntryVisibility(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantA, tenantB := isolationTenants(t, db)

	// Feedback entries require students (which require persons).
	sA := CreateTestStudentForTenant(t, db, tenantA, "FeedbackA", "Student", "2a")
	sB := CreateTestStudentForTenant(t, db, tenantB, "FeedbackB", "Student", "2a")

	feA := CreateTestFeedbackEntryForTenant(t, db, tenantA, sA.ID)
	feB := CreateTestFeedbackEntryForTenant(t, db, tenantB, sB.ID)

	repo := repoFeedback.NewEntryRepository(db)

	// --- Tenant A ---
	ctx42 := ctxForTenant(tenantA)

	entries, err := repo.List(ctx42, nil)
	require.NoError(t, err)

	for _, e := range entries {
		assert.Equal(t, tenantA, e.TenantID,
			"cross-tenant leak: tenant B feedback visible to tenant A (List)")
	}

	// FeedbackEntryRepository.FindByID returns (nil, nil) on not-found
	result, err := repo.FindByID(ctx42, feB.ID)
	assert.NoError(t, err, "EntryRepository.FindByID returns nil on not-found, not error")
	assert.Nil(t, result,
		"cross-tenant FindByID should return nil: tenant A must not see tenant B feedback %d", feB.ID)

	// --- Tenant B ---
	ctx43 := ctxForTenant(tenantB)

	entries, err = repo.List(ctx43, nil)
	require.NoError(t, err)

	for _, e := range entries {
		assert.Equal(t, tenantB, e.TenantID,
			"cross-tenant leak: tenant A feedback visible to tenant B (List)")
	}

	result, err = repo.FindByID(ctx43, feA.ID)
	assert.NoError(t, err)
	assert.Nil(t, result,
		"cross-tenant FindByID should return nil: tenant B must not see tenant A feedback %d", feA.ID)
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

	repo := repoActive.NewGroupRepository(db)

	// --- Tenant A ---
	ctx42 := ctxForTenant(tenantA)

	groups, err := repo.List(ctx42, nil)
	require.NoError(t, err)

	for _, g := range groups {
		assert.Equal(t, tenantA, g.TenantID,
			"cross-tenant leak: tenant B active group visible to tenant A (List)")
	}

	_, err = repo.FindByID(ctx42, agB.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B active group %d", agB.ID)

	// --- Tenant B ---
	ctx43 := ctxForTenant(tenantB)

	groups, err = repo.List(ctx43, nil)
	require.NoError(t, err)

	for _, g := range groups {
		assert.Equal(t, tenantB, g.TenantID,
			"cross-tenant leak: tenant A active group visible to tenant B (List)")
	}

	_, err = repo.FindByID(ctx43, agA.ID)
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

	repo := repoActivities.NewCategoryRepository(db)

	// --- Tenant A ---
	ctx42 := ctxForTenant(tenantA)

	categories, err := repo.List(ctx42, nil)
	require.NoError(t, err)

	for _, c := range categories {
		assert.Equal(t, tenantA, c.TenantID,
			"cross-tenant leak: tenant B category visible to tenant A (List)")
	}

	_, err = repo.FindByID(ctx42, catB.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B category %d", catB.ID)

	// --- Tenant B ---
	ctx43 := ctxForTenant(tenantB)

	categories, err = repo.List(ctx43, nil)
	require.NoError(t, err)

	for _, c := range categories {
		assert.Equal(t, tenantB, c.TenantID,
			"cross-tenant leak: tenant A category visible to tenant B (List)")
	}

	_, err = repo.FindByID(ctx43, catA.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant B must not see tenant A category %d", catA.ID)
}

// ============================================================================
// Audit Domain
// ============================================================================

func TestTenantIsolation_DataDeletionVisibility(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantA, tenantB := isolationTenants(t, db)

	// Data deletions require students (which require persons).
	sA := CreateTestStudentForTenant(t, db, tenantA, "AuditA", "Student", "3a")
	sB := CreateTestStudentForTenant(t, db, tenantB, "AuditB", "Student", "3a")

	ddA := CreateTestDataDeletionForTenant(t, db, tenantA, sA.ID)
	ddB := CreateTestDataDeletionForTenant(t, db, tenantB, sB.ID)

	repo := repoAudit.NewDataDeletionRepository(db)

	// --- Tenant A ---
	ctx42 := ctxForTenant(tenantA)

	deletions, err := repo.List(ctx42, nil)
	require.NoError(t, err)

	for _, d := range deletions {
		assert.Equal(t, tenantA, d.TenantID,
			"cross-tenant leak: tenant B data deletion visible to tenant A (List)")
	}

	// DataDeletionRepository uses base.Repository — FindByID returns error on not-found
	_, err = repo.FindByID(ctx42, ddB.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant A must not see tenant B data deletion %d", ddB.ID)

	// --- Tenant B ---
	ctx43 := ctxForTenant(tenantB)

	deletions, err = repo.List(ctx43, nil)
	require.NoError(t, err)

	for _, d := range deletions {
		assert.Equal(t, tenantB, d.TenantID,
			"cross-tenant leak: tenant A data deletion visible to tenant B (List)")
	}

	_, err = repo.FindByID(ctx43, ddA.ID)
	assert.Error(t, err,
		"cross-tenant FindByID should fail: tenant B must not see tenant A data deletion %d", ddA.ID)
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
	studentA := CreateTestStudentForTenant(t, db, tenantA, "WriteA", "Student", "1a")
	roomA := CreateTestRoomForTenant(t, db, tenantA, "WriteRoomA")
	groupA := CreateTestEducationGroupForTenant(t, db, tenantA, "WriteGroupA")

	ctxA := ctxForTenant(tenantA)
	ctxB := ctxForTenant(tenantB)

	// ------------------------------------------------------------------
	// 1. base.Repository.Update across 3 domains
	// ------------------------------------------------------------------

	t.Run("student update blocked", func(t *testing.T) {
		repo := repoUsers.NewStudentRepository(db)
		err := repo.Update(ctxB, studentA)
		require.Error(t, err, "cross-tenant student update must fail")
		assert.Contains(t, err.Error(), "rows affected",
			"error should mention rows affected guard")
	})

	t.Run("room update blocked", func(t *testing.T) {
		repo := repoFacilities.NewRoomRepository(db)
		err := repo.Update(ctxB, roomA)
		require.Error(t, err, "cross-tenant room update must fail")
		assert.Contains(t, err.Error(), "rows affected",
			"error should mention rows affected guard")
	})

	t.Run("education group update blocked", func(t *testing.T) {
		repo := repoEducation.NewGroupRepository(db)
		err := repo.Update(ctxB, groupA)
		require.Error(t, err, "cross-tenant group update must fail")
		assert.Contains(t, err.Error(), "rows affected",
			"error should mention rows affected guard")
	})

	// ------------------------------------------------------------------
	// 4. Custom TenantWhere + AssertRowsAffected path
	// ------------------------------------------------------------------

	t.Run("student UpdateStatus blocked", func(t *testing.T) {
		repo := repoUsers.NewStudentRepository(db)
		err := repo.UpdateStatus(ctxB, studentA.ID, users.StudentStatusInactive)
		require.Error(t, err, "cross-tenant UpdateStatus must fail")
		assert.Contains(t, err.Error(), "rows affected",
			"error should mention rows affected guard")
	})

	// ------------------------------------------------------------------
	// 5. Delete: silent no-op (no AssertRowsAffected in base.Delete)
	// ------------------------------------------------------------------

	t.Run("delete is silent no-op", func(t *testing.T) {
		repo := repoFacilities.NewRoomRepository(db)

		// Attempt cross-tenant delete: ctxB tries to delete roomA (tenant A)
		err := repo.Delete(ctxB, roomA.ID)
		assert.NoError(t, err, "cross-tenant delete should not error (silent no-op)")

		// Verify the room still exists from tenant A's perspective
		found, err := repo.FindByID(ctxA, roomA.ID)
		require.NoError(t, err, "room should still be accessible to its own tenant")
		assert.Equal(t, roomA.ID, found.ID, "room must survive cross-tenant delete attempt")
	})
}
