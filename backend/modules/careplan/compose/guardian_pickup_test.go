package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// guardianRelationshipFixture writes one People Directory relationship
// without its owner halves, the state inside the relationship's unit of work
// right before Care Plan and Identity & Access write theirs.
func guardianRelationshipFixture(t *testing.T, db *bun.DB, tenantID int64, email string) int64 {
	t.Helper()
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Pickup", "Child", "3a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Pickup", "Guardian", email)
	var relationshipID int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_guardian_relationships
		(tenant_id, student_id, guardian_profile_id, relationship_type) VALUES (?, ?, ?, 'parent') RETURNING id`,
		tenantID, student.ID, guardian.ID).Scan(context.Background(), &relationshipID))
	return relationshipID
}

func readPickupPermission(t *testing.T, db *bun.DB, relationshipID int64) (bool, *string) {
	t.Helper()
	var canPickup bool
	var notes *string
	require.NoError(t, db.NewRaw(`SELECT can_pickup, pickup_notes FROM users.student_guardian_pickup_permissions
		WHERE relationship_id = ?`, relationshipID).Scan(context.Background(), &canPickup, &notes))
	return canPickup, notes
}

func TestGuardianPickupPermissionsWriteOnlyTheirSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	var operations []string
	permissions, err := NewGuardianPickupPermissions(db, func(observation Observation) {
		operations = append(operations, observation.Operation)
	})
	require.NoError(t, err)
	relationshipID := guardianRelationshipFixture(t, db, tenantID, "pickup-owner")

	notes := "nur dienstags"
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, _ testpkg.Tx) error {
		return permissions.CreateGuardianPickupPermission(ctx, careplan.GuardianPickupPermission{
			TenantID: tenantID, RelationshipID: relationshipID, CanPickup: true, PickupNotes: &notes,
		})
	}))
	canPickup, stored := readPickupPermission(t, db, relationshipID)
	require.True(t, canPickup)
	require.Equal(t, notes, *stored)

	flag := false
	found, err := testpkg.TenantQuery1(t, ctx, db, permissions.ChangeGuardianPickupPermission, careplan.GuardianPickupPermissionChange{
		TenantID: tenantID, RelationshipID: relationshipID, CanPickup: &flag,
	})
	require.NoError(t, err)
	require.True(t, found)
	canPickup, stored = readPickupPermission(t, db, relationshipID)
	require.False(t, canPickup)
	require.Equal(t, notes, *stored, "unsupplied notes stay")
	found, err = testpkg.TenantQuery1(t, ctx, db, permissions.ChangeGuardianPickupPermission, careplan.GuardianPickupPermissionChange{
		TenantID: tenantID, RelationshipID: relationshipID, SetPickupNotes: true,
	})
	require.NoError(t, err)
	require.True(t, found)
	_, stored = readPickupPermission(t, db, relationshipID)
	require.Nil(t, stored, "supplied nil notes clear the column")

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	otherCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), otherTenantID)
	require.NoError(t, testpkg.WithTenantTx(t, otherCtx, db, otherTenantID, func(ctx context.Context, _ testpkg.Tx) error {
		_, err := testpkg.TenantQuery1(t, ctx, db, permissions.ChangeGuardianPickupPermission, careplan.GuardianPickupPermissionChange{
			TenantID: tenantID, RelationshipID: relationshipID, CanPickup: &flag,
		})
		require.ErrorIs(t, err, careplan.ErrGuardianPickupTenantMismatch, "a command naming another school is refused")
		found, err := testpkg.TenantQuery1(t, ctx, db, permissions.ChangeGuardianPickupPermission, careplan.GuardianPickupPermissionChange{
			TenantID: otherTenantID, RelationshipID: relationshipID, CanPickup: &flag,
		})
		require.NoError(t, err)
		require.False(t, found, "the other school has no such permission")
		return nil
	}))
	require.Contains(t, operations, "create_guardian_pickup_permission")
	require.Contains(t, operations, "change_guardian_pickup_permission")
}

func TestGuardianPickupPermissionsChangeRequiresExpectedState(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	permissions, err := NewGuardianPickupPermissions(db, func(Observation) {})
	require.NoError(t, err)
	relationshipID := guardianRelationshipFixture(t, db, tenantID, "pickup-expected-state")

	grant := true
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, _ testpkg.Tx) error {
		return permissions.CreateGuardianPickupPermission(ctx, careplan.GuardianPickupPermission{
			TenantID: tenantID, RelationshipID: relationshipID,
		})
	}))
	found, err := testpkg.TenantQuery1(t, ctx, db, permissions.ChangeGuardianPickupPermission, careplan.GuardianPickupPermissionChange{
		TenantID: tenantID, RelationshipID: relationshipID, CanPickup: &grant, SetWeekdays: true, Weekdays: []int{2, 4},
	})
	require.NoError(t, err)
	require.True(t, found)

	// A staff change after approval must not be overwritten by a correction
	// that still expects the approved Tuesday/Thursday permission.
	found, err = testpkg.TenantQuery1(t, ctx, db, permissions.ChangeGuardianPickupPermission, careplan.GuardianPickupPermissionChange{
		TenantID: tenantID, RelationshipID: relationshipID, SetWeekdays: true, Weekdays: []int{3},
	})
	require.NoError(t, err)
	require.True(t, found)
	revoke := false
	found, err = testpkg.TenantQuery1(t, ctx, db, permissions.ChangeGuardianPickupPermission, careplan.GuardianPickupPermissionChange{
		TenantID: tenantID, RelationshipID: relationshipID, CanPickup: &revoke, SetWeekdays: true,
		Expected: &careplan.GuardianPickupPermissionState{CanPickup: true, Weekdays: []int{2, 4}},
	})
	require.NoError(t, err)
	assert.False(t, found)

	var row struct {
		CanPickup bool  `bun:"can_pickup"`
		Weekdays  []int `bun:"pickup_weekdays,array"`
	}
	require.NoError(t, db.NewSelect().TableExpr("users.student_guardian_pickup_permissions").
		ColumnExpr("can_pickup, pickup_weekdays").Where("relationship_id = ?", relationshipID).
		Scan(ctx, &row))
	assert.True(t, row.CanPickup)
	assert.Equal(t, []int{3}, row.Weekdays)
}
