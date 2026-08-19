package audit_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestUnregisteredTagScanRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).UnregisteredTagScan
	ctx := testpkg.TenantContext(1)
	device := testpkg.CreateTestDevice(t, db, "unregistered-find")
	defer testpkg.CleanupActivityFixtures(t, db, device.ID)

	scan := createTestUnregisteredTagScan(t, repo, ctx, "TAG-FIND", &device.ID, time.Now())
	defer cleanupUnregisteredTagScans(t, db, scan.ID)

	found, err := repo.FindByID(ctx, scan.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, scan.ID, found.ID)
	require.Equal(t, "TAG-FIND", found.TagUID)
	require.Equal(t, scan.TenantID, found.SchoolID)
	require.NotEmpty(t, found.SchoolName)
	require.NotZero(t, found.OrganizationID)
	require.NotEmpty(t, found.OrganizationName)
	require.NotNil(t, found.DeviceIdentifier)
	require.Equal(t, device.DeviceID, *found.DeviceIdentifier)
	require.NotNil(t, found.DeviceName)
	require.Equal(t, "Test Device", *found.DeviceName)
}

func TestUnregisteredTagScanRepository_FindByIDReturnsNilWhenMissing(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).UnregisteredTagScan

	found, err := repo.FindByID(testpkg.TenantContext(1), int64(999999))

	require.NoError(t, err)
	require.Nil(t, found)
}

func TestUnregisteredTagScanRepository_WrapsDatabaseErrors(t *testing.T) {
	db := testpkg.SetupClosableTestDB(t)
	repo := repositories.NewFactory(db).UnregisteredTagScan
	require.NoError(t, db.Close())
	ctx := testpkg.TenantContext(1)

	found, err := repo.FindByID(ctx, int64(999999))
	require.Error(t, err)
	require.Nil(t, found)
	require.Contains(t, err.Error(), "find unregistered tag scan")

	scans, err := repo.ListForOperator(ctx, auditModels.UnregisteredTagScanFilter{})
	require.Error(t, err)
	require.Nil(t, scans)
	require.Contains(t, err.Error(), "list unregistered tag scans")

	resolved, err := repo.Resolve(ctx, int64(999999), int64(999998), nil)
	require.Error(t, err)
	require.Nil(t, resolved)
	require.Contains(t, err.Error(), "resolve unregistered tag scan")

	deleted, err := repo.DeleteOlderThan(ctx, time.Now())
	require.Error(t, err)
	require.Zero(t, deleted)
	require.Contains(t, err.Error(), "delete old unregistered tag scans")
}

func TestUnregisteredTagScanRepository_ListForOperatorFiltersAndOrders(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).UnregisteredTagScan
	ctx := testpkg.TenantContext(1)
	testpkg.EnsureTestTenant(t, db, 2)

	now := time.Now()
	oldUnresolved := createTestUnregisteredTagScan(t, repo, ctx, "TAG-OLD", nil, now.Add(-2*time.Hour))
	newUnresolved := createTestUnregisteredTagScan(t, repo, ctx, "TAG-NEW", nil, now.Add(-time.Hour))
	resolved := createTestUnregisteredTagScan(t, repo, ctx, "TAG-RESOLVED", nil, now)
	otherTenant := createTestUnregisteredTagScan(t, repo, testpkg.TenantContext(2), "TAG-OTHER-TENANT", nil, now)
	defer cleanupUnregisteredTagScans(t, db, oldUnresolved.ID, newUnresolved.ID, resolved.ID, otherTenant.ID)

	resolvedAt := now
	_, err := db.NewUpdate().
		Model((*auditModels.UnregisteredTagScan)(nil)).
		ModelTableExpr("audit.unregistered_tag_scans").
		Set("resolved_at = ?", resolvedAt).
		Where("id = ?", resolved.ID).
		Exec(context.Background())
	require.NoError(t, err)

	schoolID := newUnresolved.TenantID
	orgID := organizationIDForSchool(t, db, schoolID)
	scans, err := repo.ListForOperator(ctx, auditModels.UnregisteredTagScanFilter{
		SchoolID:       &schoolID,
		OrganizationID: &orgID,
		UnresolvedOnly: true,
	})

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(scans), 2)
	require.Equal(t, newUnresolved.ID, scans[0].ID)
	require.Equal(t, oldUnresolved.ID, scans[1].ID)
	for _, scan := range scans {
		require.Equal(t, schoolID, scan.SchoolID)
		require.Nil(t, scan.ResolvedAt)
	}
}

func TestUnregisteredTagScanRepository_ListForOperatorReturnsEmptySlice(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).UnregisteredTagScan
	schoolID := int64(999998)

	scans, err := repo.ListForOperator(testpkg.TenantContext(1), auditModels.UnregisteredTagScanFilter{
		SchoolID: &schoolID,
	})

	require.NoError(t, err)
	require.NotNil(t, scans)
	require.Empty(t, scans)
}

func TestUnregisteredTagScanRepository_Resolve(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).UnregisteredTagScan
	ctx := testpkg.TenantContext(1)
	operator := createAuditTestOperator(t, db)
	defer cleanupAuditTestOperator(t, db, operator.ID)

	scan := createTestUnregisteredTagScan(t, repo, ctx, "TAG-RESOLVE", nil, time.Now())
	defer cleanupUnregisteredTagScans(t, db, scan.ID)
	note := "Issued replacement card"

	resolved, err := repo.Resolve(ctx, scan.ID, operator.ID, &note)

	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.Equal(t, scan.ID, resolved.ID)
	require.NotNil(t, resolved.ResolvedAt)
	require.NotNil(t, resolved.ResolvedByOperatorID)
	require.Equal(t, operator.ID, *resolved.ResolvedByOperatorID)
	require.NotNil(t, resolved.ResolutionNote)
	require.Equal(t, note, *resolved.ResolutionNote)
}

func TestUnregisteredTagScanRepository_ResolveFailsWhenAlreadyResolved(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).UnregisteredTagScan
	ctx := testpkg.TenantContext(1)
	operator := createAuditTestOperator(t, db)
	defer cleanupAuditTestOperator(t, db, operator.ID)
	scan := createTestUnregisteredTagScan(t, repo, ctx, "TAG-RESOLVE-ONCE", nil, time.Now())
	defer cleanupUnregisteredTagScans(t, db, scan.ID)

	_, err := repo.Resolve(ctx, scan.ID, operator.ID, nil)
	require.NoError(t, err)

	_, err = repo.Resolve(ctx, scan.ID, operator.ID, nil)

	require.Error(t, err)
}

func TestUnregisteredTagScanRepository_DeleteOlderThan(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).UnregisteredTagScan
	ctx := testpkg.TenantContext(1)
	now := time.Now()
	oldScan := createTestUnregisteredTagScan(t, repo, ctx, "TAG-DELETE-OLD", nil, now.Add(-48*time.Hour))
	newScan := createTestUnregisteredTagScan(t, repo, ctx, "TAG-DELETE-NEW", nil, now)
	defer cleanupUnregisteredTagScans(t, db, oldScan.ID, newScan.ID)

	deleted, err := repo.DeleteOlderThan(ctx, now.Add(-24*time.Hour))

	require.NoError(t, err)
	require.Equal(t, 1, deleted)

	foundOld, err := repo.FindByID(ctx, oldScan.ID)
	require.NoError(t, err)
	require.Nil(t, foundOld)

	foundNew, err := repo.FindByID(ctx, newScan.ID)
	require.NoError(t, err)
	require.NotNil(t, foundNew)
}

func createTestUnregisteredTagScan(
	t *testing.T,
	repo auditModels.UnregisteredTagScanRepository,
	ctx context.Context,
	tagUID string,
	deviceID *int64,
	scannedAt time.Time,
) *auditModels.UnregisteredTagScan {
	t.Helper()

	scan := &auditModels.UnregisteredTagScan{
		TagUID:    tagUID,
		DeviceID:  deviceID,
		ScannedAt: scannedAt,
	}
	require.NoError(t, repo.Create(ctx, scan))
	require.NotZero(t, scan.ID)
	return scan
}

func createAuditTestOperator(t *testing.T, db *bun.DB) *platformModels.Operator {
	t.Helper()

	operator := &platformModels.Operator{
		Email:        fmt.Sprintf("audit-scan-%d@example.com", time.Now().UnixNano()),
		DisplayName:  "Audit Scan Operator",
		PasswordHash: "hashed-password",
		Active:       true,
	}
	require.NoError(t, db.NewInsert().
		Model(operator).
		ModelTableExpr("platform.operators").
		Scan(context.Background()))
	require.NotZero(t, operator.ID)
	return operator
}

func organizationIDForSchool(t *testing.T, db *bun.DB, schoolID int64) int64 {
	t.Helper()

	var orgID int64
	require.NoError(t, db.NewRaw(
		"SELECT organization_id FROM platform.schools WHERE id = ?",
		schoolID,
	).Scan(context.Background(), &orgID))
	require.NotZero(t, orgID)
	return orgID
}

func cleanupUnregisteredTagScans(t *testing.T, db *bun.DB, ids ...int64) {
	t.Helper()
	testpkg.CleanupTableRecords(t, db, "audit.unregistered_tag_scans", ids...)
}

func cleanupAuditTestOperator(t *testing.T, db *bun.DB, operatorID int64) {
	t.Helper()
	testpkg.CleanupTableRecords(t, db, "platform.operators", operatorID)
}
