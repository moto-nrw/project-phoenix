package audit

import (
	"context"
	"fmt"
	"testing"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestUnregisteredTagScanRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewUnregisteredTagScanRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)
	device := testpkg.CreateTestDevice(t, db, "unregistered-find")

	scan := createTestUnregisteredTagScan(t, repo, ctx, "TAG-FIND", &device.ID, time.Now())

	found, err := repo.FindByID(ctx, scan.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, scan.ID, found.ID)
	require.Equal(t, "TAG-FIND", found.TagUID)
	require.Equal(t, scan.TenantID, found.SchoolID)
	require.Empty(t, found.SchoolName, "school names are resolved by the owner capability in the service")
	require.Zero(t, found.OrganizationID, "organization ownership is resolved by the owner capability in the service")
	require.Empty(t, found.OrganizationName, "organization names are resolved by the owner capability in the service")
	require.NotNil(t, found.DeviceIdentifier)
	require.Equal(t, device.DeviceID, *found.DeviceIdentifier)
	require.NotNil(t, found.DeviceName)
	require.Equal(t, "Test Device", *found.DeviceName)
}

func TestUnregisteredTagScanRepository_FindByIDReturnsNilWhenMissing(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewUnregisteredTagScanRepository(NewRuntime(db, auditTestTenantID))

	found, err := repo.FindByID(testpkg.Ctx(t), int64(999999))

	require.NoError(t, err)
	require.Nil(t, found)
}

func TestUnregisteredTagScanRepository_WrapsDatabaseErrors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupClosableTestDB(t)
	repo := NewUnregisteredTagScanRepository(NewRuntime(db, auditTestTenantID))
	require.NoError(t, db.Close())
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewUnregisteredTagScanRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)
	now := time.Now()
	oldUnresolved := createTestUnregisteredTagScan(t, repo, ctx, "TAG-OLD", nil, now.Add(-2*time.Hour))
	newUnresolved := createTestUnregisteredTagScan(t, repo, ctx, "TAG-NEW", nil, now.Add(-time.Hour))
	resolved := createTestUnregisteredTagScan(t, repo, ctx, "TAG-RESOLVED", nil, now)
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	createTestUnregisteredTagScan(t, repo, testpkg.TenantContext(otherTenantID), "TAG-OTHER-TENANT", nil, now)

	resolvedAt := now
	_, err := db.NewUpdate().
		Model((*auditModels.UnregisteredTagScan)(nil)).
		ModelTableExpr("audit.unregistered_tag_scans").
		Set("resolved_at = ?", resolvedAt).
		Where("id = ?", resolved.ID).
		Exec(context.Background())
	require.NoError(t, err)

	schoolID := newUnresolved.TenantID
	orgID := testpkg.OrganizationIDForSchool(t, db, schoolID)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewUnregisteredTagScanRepository(NewRuntime(db, auditTestTenantID))
	schoolID := int64(999998)

	scans, err := repo.ListForOperator(testpkg.Ctx(t), auditModels.UnregisteredTagScanFilter{
		SchoolID: &schoolID,
	})

	require.NoError(t, err)
	require.NotNil(t, scans)
	require.Empty(t, scans)
}

func TestUnregisteredTagScanRepository_Resolve(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewUnregisteredTagScanRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperatorWithEmail(t, db,
		fmt.Sprintf("audit-scan-%d@example.com", time.Now().UnixNano()), "Audit Scan Operator")

	scan := createTestUnregisteredTagScan(t, repo, ctx, "TAG-RESOLVE", nil, time.Now())
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewUnregisteredTagScanRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperatorWithEmail(t, db,
		fmt.Sprintf("audit-scan-%d@example.com", time.Now().UnixNano()), "Audit Scan Operator")
	scan := createTestUnregisteredTagScan(t, repo, ctx, "TAG-RESOLVE-ONCE", nil, time.Now())

	_, err := repo.Resolve(ctx, scan.ID, operator.ID, nil)
	require.NoError(t, err)

	_, err = repo.Resolve(ctx, scan.ID, operator.ID, nil)

	require.Error(t, err)
}

func TestUnregisteredTagScanRepository_DeleteOlderThan(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewUnregisteredTagScanRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)
	now := time.Now()
	oldScan := createTestUnregisteredTagScan(t, repo, ctx, "TAG-DELETE-OLD", nil, now.Add(-120*24*time.Hour))
	newScan := createTestUnregisteredTagScan(t, repo, ctx, "TAG-DELETE-NEW", nil, now)

	deleted, err := repo.DeleteOlderThan(ctx, now.Add(-90*24*time.Hour))

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
