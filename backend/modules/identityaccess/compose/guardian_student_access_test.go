package compose

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func readGuardianStudentAccess(t *testing.T, db *bun.DB, relationshipID int64) (*int64, map[string]any) {
	t.Helper()
	var accountID *int64
	var permissions map[string]any
	require.NoError(t, db.NewRaw(`SELECT account_id, permissions FROM auth.guardian_student_access WHERE relationship_id = ?`, relationshipID).
		Scan(context.Background(), &accountID, &permissions))
	return accountID, permissions
}

func TestGuardianStudentAccessWritesOnlyItsSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	var operations []string
	access, err := NewGuardianStudentAccess(db, func(observation Observation) {
		operations = append(operations, observation.Operation)
	})
	require.NoError(t, err)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Access", "Child", "3a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Access", "Guardian", "access-owner")
	account := testpkg.CreateTestAccount(t, db, "access-owner-account")
	_, err = db.ExecContext(ctx, `UPDATE users.guardian_profiles SET account_id = ?, has_account = true WHERE id = ?`, account.ID, guardian.ID)
	require.NoError(t, err)
	var relationshipID int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.student_guardian_relationships
		(tenant_id, student_id, guardian_profile_id, relationship_type) VALUES (?, ?, ?, 'parent') RETURNING id`,
		tenantID, student.ID, guardian.ID).Scan(ctx, &relationshipID))

	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, _ testpkg.Tx) error {
		return access.GrantGuardianStudentAccess(ctx, identityaccess.GuardianStudentAccessGrant{
			TenantID: tenantID, RelationshipID: relationshipID, AccountID: &account.ID,
			Permissions: json.RawMessage(`{"parent_portal.access": true}`),
		})
	}))
	boundAccount, permissions := readGuardianStudentAccess(t, db, relationshipID)
	require.NotNil(t, boundAccount)
	require.Equal(t, account.ID, *boundAccount)
	require.Equal(t, map[string]any{"parent_portal.access": true}, permissions)

	found, err := access.SetGuardianStudentPermissions(ctx, tenantID, relationshipID, nil)
	require.NoError(t, err)
	require.True(t, found)
	_, permissions = readGuardianStudentAccess(t, db, relationshipID)
	require.Empty(t, permissions, "empty permissions are stored as {}")

	// Unlinking the portal account on the profile clears the binding.
	_, err = db.ExecContext(ctx, `UPDATE users.guardian_profiles SET account_id = NULL, has_account = false WHERE id = ?`, guardian.ID)
	require.NoError(t, err)
	boundAccount, _ = readGuardianStudentAccess(t, db, relationshipID)
	require.Nil(t, boundAccount)

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	otherCtx := testpkg.ContextForTenant(testpkg.WithPackageTenantRuntime(context.Background()), otherTenantID)
	require.NoError(t, testpkg.WithTenantTx(t, otherCtx, db, otherTenantID, func(ctx context.Context, _ testpkg.Tx) error {
		_, err := access.SetGuardianStudentPermissions(ctx, tenantID, relationshipID, json.RawMessage(`{"parent_portal.access": true}`))
		require.ErrorIs(t, err, identityaccess.ErrGuardianStudentAccessTenantMismatch, "a command naming another school is refused")
		found, err := access.SetGuardianStudentPermissions(ctx, otherTenantID, relationshipID, json.RawMessage(`{"parent_portal.access": true}`))
		require.NoError(t, err)
		require.False(t, found, "the other school has no such access row")
		return nil
	}))
	_, permissions = readGuardianStudentAccess(t, db, relationshipID)
	require.Empty(t, permissions)
	require.Contains(t, operations, "grant_guardian_student_access")
	require.Contains(t, operations, "set_guardian_student_permissions")
}
