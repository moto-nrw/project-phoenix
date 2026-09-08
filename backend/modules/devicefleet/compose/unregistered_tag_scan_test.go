package compose_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// newScanFleet composes the real Device Fleet owner (#2678). Every
// unregistered-tag-scan read and write below goes through its public facade,
// which is the only provider over audit.unregistered_tag_scans.
func newScanFleet(t *testing.T, db *bun.DB) devicefleet.Capability {
	t.Helper()
	fleet, err := repositories.NewDeviceFleet(db)
	require.NoError(t, err, "Failed to compose the device fleet")
	return fleet
}

func recordScan(t *testing.T, fleet devicefleet.Capability, ctx context.Context, tagUID string, deviceID *int64, scannedAt time.Time) devicefleet.UnregisteredTagScan {
	t.Helper()
	scan, err := fleet.RecordUnregisteredTagScan(ctx, devicefleet.RecordUnregisteredTagScan{
		TagUID: tagUID, DeviceID: deviceID, ScannedAt: scannedAt,
	})
	require.NoError(t, err)
	require.NotZero(t, scan.ID)
	return scan
}

func newScanOperator(t *testing.T, db *bun.DB) int64 {
	t.Helper()
	operator := testpkg.CreateTestOperatorWithEmail(t, db,
		fmt.Sprintf("scan-review-%d@example.com", time.Now().UnixNano()), "Scan Review Operator")
	return operator.ID
}

func TestUnregisteredTagScan_RecordAttachesOwnDeviceIdentity(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	fleet := newScanFleet(t, db)
	ctx := testpkg.Ctx(t)
	device := testpkg.CreateTestDevice(t, db, "scan-record")

	recorded := recordScan(t, fleet, ctx, "  TAG-RECORD  ", &device.ID, time.Time{})

	require.Equal(t, "TAG-RECORD", recorded.TagUID, "the owner trims the tag UID")
	require.Equal(t, testpkg.Tenant(t), recorded.TenantID)
	require.False(t, recorded.ScannedAt.IsZero(), "a zero scan time is stamped by the owner clock")

	found, err := fleet.FindUnregisteredTagScan(ctx, recorded.ID)
	require.NoError(t, err)
	require.Equal(t, recorded.ID, found.ID)
	require.Equal(t, testpkg.Tenant(t), found.TenantID)
	require.Nil(t, found.ResolvedAt)
	require.NotNil(t, found.DeviceIdentifier)
	assert.Equal(t, device.DeviceID, *found.DeviceIdentifier)
	require.NotNil(t, found.DeviceName)
	assert.Equal(t, "Test Device", *found.DeviceName)
}

func TestUnregisteredTagScan_RecordRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	fleet := newScanFleet(t, db)
	ctx := testpkg.Ctx(t)

	_, err := fleet.RecordUnregisteredTagScan(ctx, devicefleet.RecordUnregisteredTagScan{TagUID: "   "})
	require.ErrorIs(t, err, devicefleet.ErrInvalidUnregisteredTagScan)

	zero := int64(0)
	_, err = fleet.RecordUnregisteredTagScan(ctx, devicefleet.RecordUnregisteredTagScan{TagUID: "TAG", DeviceID: &zero})
	require.ErrorIs(t, err, devicefleet.ErrInvalidUnregisteredTagScan)

	_, err = fleet.RecordUnregisteredTagScan(testpkg.WithTestTenantRuntime(t, context.Background()), devicefleet.RecordUnregisteredTagScan{TagUID: "TAG"})
	require.ErrorIs(t, err, devicefleet.ErrTenantRequired, "a scan without a tenant is never appended")
}

func TestUnregisteredTagScan_FindReportsNotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	fleet := newScanFleet(t, db)

	_, err := fleet.FindUnregisteredTagScan(testpkg.Ctx(t), int64(999999))
	require.ErrorIs(t, err, devicefleet.ErrUnregisteredTagScanNotFound)

	_, err = fleet.FindUnregisteredTagScan(testpkg.Ctx(t), 0)
	require.ErrorIs(t, err, devicefleet.ErrInvalidUnregisteredTagScan)
}

func TestUnregisteredTagScan_ListFiltersAndOrders(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	fleet := newScanFleet(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	operatorID := newScanOperator(t, db)
	now := time.Now()
	oldUnresolved := recordScan(t, fleet, ctx, "TAG-OLD", nil, now.Add(-2*time.Hour))
	newUnresolved := recordScan(t, fleet, ctx, "TAG-NEW", nil, now.Add(-time.Hour))
	resolved := recordScan(t, fleet, ctx, "TAG-RESOLVED", nil, now)
	require.NoError(t, testpkg.WithinAdminContext(t, ctx, db, func(adminCtx context.Context) error {
		_, err := fleet.ResolveUnregisteredTagScan(adminCtx, devicefleet.ResolveUnregisteredTagScan{ID: resolved.ID, OperatorID: operatorID})
		return err
	}))

	scans, err := fleet.ListUnregisteredTagScans(ctx, devicefleet.UnregisteredTagScanFilter{
		TenantIDs: []int64{tenantID}, UnresolvedOnly: true,
	})
	require.NoError(t, err)
	require.Len(t, scans, 2)
	assert.Equal(t, newUnresolved.ID, scans[0].ID, "newest scan first")
	assert.Equal(t, oldUnresolved.ID, scans[1].ID)
	for _, scan := range scans {
		assert.Equal(t, tenantID, scan.TenantID)
		assert.Nil(t, scan.ResolvedAt)
	}

	all, err := fleet.ListUnregisteredTagScans(ctx, devicefleet.UnregisteredTagScanFilter{TenantIDs: []int64{tenantID}})
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.Equal(t, resolved.ID, all[0].ID)

	limited, err := fleet.ListUnregisteredTagScans(ctx, devicefleet.UnregisteredTagScanFilter{TenantIDs: []int64{tenantID}, Limit: 1})
	require.NoError(t, err)
	require.Len(t, limited, 1)

	none, err := fleet.ListUnregisteredTagScans(ctx, devicefleet.UnregisteredTagScanFilter{TenantIDs: []int64{}})
	require.NoError(t, err)
	require.NotNil(t, none)
	assert.Empty(t, none, "an empty tenant list matches nothing")

	_, err = fleet.ListUnregisteredTagScans(ctx, devicefleet.UnregisteredTagScanFilter{TenantIDs: []int64{0}})
	require.ErrorIs(t, err, devicefleet.ErrInvalidUnregisteredTagScan)
}

func TestUnregisteredTagScan_ResolveStampsOnce(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	fleet := newScanFleet(t, db)
	ctx := testpkg.Ctx(t)
	operatorID := newScanOperator(t, db)
	scan := recordScan(t, fleet, ctx, "TAG-RESOLVE", nil, time.Now())
	note := "  Issued replacement card  "

	var resolved devicefleet.UnregisteredTagScan
	require.NoError(t, testpkg.WithinAdminContext(t, ctx, db, func(adminCtx context.Context) error {
		var err error
		resolved, err = fleet.ResolveUnregisteredTagScan(adminCtx, devicefleet.ResolveUnregisteredTagScan{
			ID: scan.ID, OperatorID: operatorID, Note: &note,
		})
		return err
	}))
	require.Equal(t, scan.ID, resolved.ID)
	require.NotNil(t, resolved.ResolvedAt)
	require.NotNil(t, resolved.ResolvedByOperatorID)
	assert.Equal(t, operatorID, *resolved.ResolvedByOperatorID)
	require.NotNil(t, resolved.ResolutionNote)
	assert.Equal(t, "Issued replacement card", *resolved.ResolutionNote, "the note is trimmed")

	err := testpkg.WithinAdminContext(t, ctx, db, func(adminCtx context.Context) error {
		_, err := fleet.ResolveUnregisteredTagScan(adminCtx, devicefleet.ResolveUnregisteredTagScan{ID: scan.ID, OperatorID: operatorID})
		return err
	})
	require.ErrorIs(t, err, devicefleet.ErrUnregisteredTagScanResolved, "a retry never double-stamps")

	missingID := int64(999999)
	err = testpkg.WithinAdminContext(t, ctx, db, func(adminCtx context.Context) error {
		_, err := fleet.ResolveUnregisteredTagScan(adminCtx, devicefleet.ResolveUnregisteredTagScan{ID: missingID, OperatorID: operatorID})
		return err
	})
	require.ErrorIs(t, err, devicefleet.ErrUnregisteredTagScanNotFound)

	_, err = fleet.ResolveUnregisteredTagScan(ctx, devicefleet.ResolveUnregisteredTagScan{ID: scan.ID})
	require.ErrorIs(t, err, devicefleet.ErrInvalidUnregisteredTagScan)
}

func TestUnregisteredTagScan_ResolveKeepsBlankNoteEmpty(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	fleet := newScanFleet(t, db)
	ctx := testpkg.Ctx(t)
	operatorID := newScanOperator(t, db)
	scan := recordScan(t, fleet, ctx, "TAG-BLANK-NOTE", nil, time.Now())
	blank := "   "

	var resolved devicefleet.UnregisteredTagScan
	require.NoError(t, testpkg.WithinAdminContext(t, ctx, db, func(adminCtx context.Context) error {
		var err error
		resolved, err = fleet.ResolveUnregisteredTagScan(adminCtx, devicefleet.ResolveUnregisteredTagScan{
			ID: scan.ID, OperatorID: operatorID, Note: &blank,
		})
		return err
	}))
	assert.Nil(t, resolved.ResolutionNote)
}

func TestUnregisteredTagScan_DeleteExpiredKeepsRecentScans(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	fleet := newScanFleet(t, db)
	ctx := testpkg.Ctx(t)
	now := time.Now()
	oldScan := recordScan(t, fleet, ctx, "TAG-EXPIRED", nil, now.Add(-120*24*time.Hour))
	newScan := recordScan(t, fleet, ctx, "TAG-KEPT", nil, now)

	deleted, err := fleet.DeleteExpiredUnregisteredTagScans(ctx, now.Add(-devicefleet.UnregisteredTagScanRetentionDays*24*time.Hour))
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted, "only the expired scan is deleted")

	_, err = fleet.FindUnregisteredTagScan(ctx, oldScan.ID)
	require.ErrorIs(t, err, devicefleet.ErrUnregisteredTagScanNotFound)
	_, err = fleet.FindUnregisteredTagScan(ctx, newScan.ID)
	require.NoError(t, err)

	_, err = fleet.DeleteExpiredUnregisteredTagScans(ctx, time.Time{})
	require.ErrorIs(t, err, devicefleet.ErrInvalidUnregisteredTagScan)

	_, err = fleet.DeleteExpiredUnregisteredTagScans(testpkg.WithTestTenantRuntime(t, context.Background()), now)
	require.ErrorIs(t, err, devicefleet.ErrTenantRequired, "retention never runs without a tenant")
}

// TestUnregisteredTagScan_TwoTenantIsolation proves audit.unregistered_tag_scans
// is bounded by RLS for the tenant role and that the admin review still sees
// each tenant's rows with only that tenant's device identity.
func TestUnregisteredTagScan_TwoTenantIsolation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	fleet := newScanFleet(t, db)
	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)
	deviceA := testpkg.CreateTestDeviceForTenant(t, db, tenantA, "scan-iso-a")
	operatorID := newScanOperator(t, db)
	base := testpkg.WithTestTenantRuntime(t, context.Background())

	var scanA, scanB devicefleet.UnregisteredTagScan
	require.NoError(t, testpkg.WithinTenantContext(t, base, db, tenantA, func(ctxA context.Context) error {
		scanA = recordScan(t, fleet, ctxA, "TAG-A", &deviceA.ID, time.Now())
		return nil
	}))
	require.NoError(t, testpkg.WithinTenantContext(t, base, db, tenantB, func(ctxB context.Context) error {
		// Tenant B's device column may point at another tenant's device row;
		// the foreign key is not tenant-bound. The owner must not reveal it.
		scanB = recordScan(t, fleet, ctxB, "TAG-B", &deviceA.ID, time.Now())
		return nil
	}))
	require.Equal(t, tenantA, scanA.TenantID)
	require.Equal(t, tenantB, scanB.TenantID)

	require.NoError(t, testpkg.WithinTenantContext(t, base, db, tenantB, func(ctxB context.Context) error {
		_, err := fleet.FindUnregisteredTagScan(ctxB, scanA.ID)
		assert.ErrorIs(t, err, devicefleet.ErrUnregisteredTagScanNotFound, "tenant B must not see tenant A's scan")

		scans, err := fleet.ListUnregisteredTagScans(ctxB, devicefleet.UnregisteredTagScanFilter{TenantIDs: []int64{tenantA, tenantB}})
		require.NoError(t, err)
		require.Len(t, scans, 1)
		assert.Equal(t, scanB.ID, scans[0].ID)
		assert.Nil(t, scans[0].DeviceIdentifier, "tenant A's device identity never crosses into tenant B")
		assert.Nil(t, scans[0].DeviceName)
		return nil
	}))

	require.NoError(t, testpkg.WithinAdminContext(t, base, db, func(adminCtx context.Context) error {
		scans, err := fleet.ListUnregisteredTagScans(adminCtx, devicefleet.UnregisteredTagScanFilter{TenantIDs: []int64{tenantA, tenantB}})
		require.NoError(t, err)
		require.Len(t, scans, 2, "the operator review sees both tenants")
		byID := make(map[int64]devicefleet.UnregisteredTagScan, len(scans))
		for _, scan := range scans {
			byID[scan.ID] = scan
		}
		require.NotNil(t, byID[scanA.ID].DeviceIdentifier)
		assert.Equal(t, deviceA.DeviceID, *byID[scanA.ID].DeviceIdentifier)
		assert.Nil(t, byID[scanB.ID].DeviceIdentifier, "a device of another tenant leaves the identity unset")

		_, err = fleet.ResolveUnregisteredTagScan(adminCtx, devicefleet.ResolveUnregisteredTagScan{ID: scanA.ID, OperatorID: operatorID})
		require.NoError(t, err)
		return nil
	}))

	require.NoError(t, testpkg.WithinTenantContext(t, base, db, tenantB, func(ctxB context.Context) error {
		scans, err := fleet.ListUnregisteredTagScans(ctxB, devicefleet.UnregisteredTagScanFilter{UnresolvedOnly: true})
		require.NoError(t, err)
		for _, scan := range scans {
			assert.Equal(t, tenantB, scan.TenantID, "cross-tenant leak: tenant A scan visible to tenant B")
		}
		deleted, err := fleet.DeleteExpiredUnregisteredTagScans(ctxB, time.Now().Add(-devicefleet.UnregisteredTagScanRetentionDays*24*time.Hour))
		require.NoError(t, err)
		assert.Zero(t, deleted, "retention is scoped to the caller tenant")
		return nil
	}))
}

// TestUnregisteredTagScan_RecordRollsBackWithOuterUnitOfWork injects a failure
// after the authoritative insert and proves the row is gone and that a retry
// appends exactly one row.
func TestUnregisteredTagScan_RecordRollsBackWithOuterUnitOfWork(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	fleet := newScanFleet(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	tagUID := fmt.Sprintf("TAG-ROLLBACK-%d", time.Now().UnixNano())
	wantErr := errors.New("abort after append")

	var aborted devicefleet.UnregisteredTagScan
	err := testpkg.WithinTenantContext(t, ctx, db, tenantID, func(txCtx context.Context) error {
		aborted = recordScan(t, fleet, txCtx, tagUID, nil, time.Now())
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	_, err = fleet.FindUnregisteredTagScan(ctx, aborted.ID)
	require.ErrorIs(t, err, devicefleet.ErrUnregisteredTagScanNotFound, "the aborted append left no row")

	retried := recordScan(t, fleet, ctx, tagUID, nil, time.Now())
	scans, err := fleet.ListUnregisteredTagScans(ctx, devicefleet.UnregisteredTagScanFilter{TenantIDs: []int64{tenantID}})
	require.NoError(t, err)
	matches := 0
	for _, scan := range scans {
		if scan.TagUID == tagUID {
			matches++
			assert.Equal(t, retried.ID, scan.ID)
		}
	}
	assert.Equal(t, 1, matches, "the retry appended exactly one row")
}

// TestUnregisteredTagScan_ResolveRollsBackWithOuterUnitOfWork injects a
// failure after the authoritative update and proves the scan stays open and
// that the retry resolves it exactly once.
func TestUnregisteredTagScan_ResolveRollsBackWithOuterUnitOfWork(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	fleet := newScanFleet(t, db)
	ctx := testpkg.Ctx(t)
	operatorID := newScanOperator(t, db)
	scan := recordScan(t, fleet, ctx, "TAG-RESOLVE-ROLLBACK", nil, time.Now())
	wantErr := errors.New("abort after resolve")

	err := testpkg.WithinAdminContext(t, ctx, db, func(adminCtx context.Context) error {
		resolved, resolveErr := fleet.ResolveUnregisteredTagScan(adminCtx, devicefleet.ResolveUnregisteredTagScan{ID: scan.ID, OperatorID: operatorID})
		require.NoError(t, resolveErr)
		require.NotNil(t, resolved.ResolvedAt)
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	found, err := fleet.FindUnregisteredTagScan(ctx, scan.ID)
	require.NoError(t, err)
	assert.Nil(t, found.ResolvedAt, "the aborted resolution left the scan open")

	require.NoError(t, testpkg.WithinAdminContext(t, ctx, db, func(adminCtx context.Context) error {
		_, err := fleet.ResolveUnregisteredTagScan(adminCtx, devicefleet.ResolveUnregisteredTagScan{ID: scan.ID, OperatorID: operatorID})
		return err
	}))
	err = testpkg.WithinAdminContext(t, ctx, db, func(adminCtx context.Context) error {
		_, err := fleet.ResolveUnregisteredTagScan(adminCtx, devicefleet.ResolveUnregisteredTagScan{ID: scan.ID, OperatorID: operatorID})
		return err
	})
	require.ErrorIs(t, err, devicefleet.ErrUnregisteredTagScanResolved)
}

func TestUnregisteredTagScan_KeepsPersistenceFailuresVisible(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupClosableTestDB(t)
	fleet := newScanFleet(t, db)
	ctx := testpkg.Ctx(t)
	require.NoError(t, db.Close())

	_, err := fleet.FindUnregisteredTagScan(ctx, int64(999999))
	require.Error(t, err)
	assert.NotErrorIs(t, err, devicefleet.ErrUnregisteredTagScanNotFound, "a read failure is never reported as a missing scan")
	assert.Contains(t, err.Error(), "find unregistered tag scan")

	_, err = fleet.ListUnregisteredTagScans(ctx, devicefleet.UnregisteredTagScanFilter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list unregistered tag scans")

	// Writes open their transaction through the ambient unit of work, which
	// the test harness binds to the package pool rather than this closed
	// handle; their failure paths are covered by the rollback tests above.
}
