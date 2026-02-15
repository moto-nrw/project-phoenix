// Package test provides tenant isolation tests that verify cross-tenant data
// cannot leak between different schools (tenants) in the multi-tenant system.
//
// WP 3.19: These tests exercise the defense-in-depth WHERE tenant_id = ? filters
// applied by every tenant-scoped repository. Each test creates data for two
// tenants (IDs 42 and 43), then verifies that List and FindByID respect
// tenant boundaries in both directions.
package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repoAuth "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	repoConfig "github.com/moto-nrw/project-phoenix/database/repositories/config"
	repoEducation "github.com/moto-nrw/project-phoenix/database/repositories/education"
	repoFacilities "github.com/moto-nrw/project-phoenix/database/repositories/facilities"
	repoFeedback "github.com/moto-nrw/project-phoenix/database/repositories/feedback"
	repoIot "github.com/moto-nrw/project-phoenix/database/repositories/iot"
	repoSchedule "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	repoUsers "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Tenant IDs used throughout isolation tests.
// Values >= 42 satisfy the hermetic test scanner (int64(1)..int64(9) are flagged).
const (
	tenantA = int64(42)
	tenantB = int64(43)
)

// ctxForTenant returns a background context with the given tenant ID set.
func ctxForTenant(tenantID int64) context.Context {
	return tenant.WithTenantID(context.Background(), tenantID)
}

// ============================================================================
// Users Domain
// ============================================================================

func TestTenantIsolation_StudentVisibility(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()
	defer CleanupTenantTestData(t, db, tenantA, tenantB)

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
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()
	defer CleanupTenantTestData(t, db, tenantA, tenantB)

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
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()
	defer CleanupTenantTestData(t, db, tenantA, tenantB)

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
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()
	defer CleanupTenantTestData(t, db, tenantA, tenantB)

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
// Config Domain
// ============================================================================

func TestTenantIsolation_SettingVisibility(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()
	defer CleanupTenantTestData(t, db, tenantA, tenantB)

	setA := CreateTestSettingForTenant(t, db, tenantA, "test_key_a", "valueA", "general")
	setB := CreateTestSettingForTenant(t, db, tenantB, "test_key_b", "valueB", "general")

	repo := repoConfig.NewSettingRepository(db)

	// --- Tenant A ---
	ctx42 := ctxForTenant(tenantA)

	settings, err := repo.List(ctx42, nil)
	require.NoError(t, err)

	for _, s := range settings {
		assert.Equal(t, tenantA, s.TenantID,
			"cross-tenant leak: tenant B setting visible to tenant A (List)")
	}

	// SettingRepository.FindByID returns (nil, nil) on not-found instead of error
	result, err := repo.FindByID(ctx42, setB.ID)
	assert.NoError(t, err, "SettingRepository.FindByID returns nil on not-found, not error")
	assert.Nil(t, result,
		"cross-tenant FindByID should return nil: tenant A must not see tenant B setting %d", setB.ID)

	// --- Tenant B ---
	ctx43 := ctxForTenant(tenantB)

	settings, err = repo.List(ctx43, nil)
	require.NoError(t, err)

	for _, s := range settings {
		assert.Equal(t, tenantB, s.TenantID,
			"cross-tenant leak: tenant A setting visible to tenant B (List)")
	}

	result, err = repo.FindByID(ctx43, setA.ID)
	assert.NoError(t, err)
	assert.Nil(t, result,
		"cross-tenant FindByID should return nil: tenant B must not see tenant A setting %d", setA.ID)
}

// ============================================================================
// IoT Domain
// ============================================================================

func TestTenantIsolation_DeviceVisibility(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()
	defer CleanupTenantTestData(t, db, tenantA, tenantB)

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
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()
	defer CleanupTenantTestData(t, db, tenantA, tenantB)

	// Tokens require an account (accounts are not tenant-scoped).
	acctA := CreateTestAccount(t, db, "token-isolation-a")
	acctB := CreateTestAccount(t, db, "token-isolation-b")
	defer CleanupAuthFixtures(t, db, acctA.ID, acctB.ID)

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
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()
	defer CleanupTenantTestData(t, db, tenantA, tenantB)

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
