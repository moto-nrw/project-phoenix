package parentrequestfeedprojection

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestResolveAccessRequiresSchoolWideRightsAndActiveMembership(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "request-feed-access@example.test")
	tenantID := testpkg.Tenant(t)

	var access Access
	var found bool
	err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		var resolveErr error
		access, found, resolveErr = ResolveAccess(ctx, tx, tenantID, account.ID)
		return resolveErr
	})
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, access.GeneralRequests, "a plain account must not get a school-wide feed")
	require.False(t, access.EnrollmentRequests)

	result, err := db.ExecContext(testpkg.Ctx(t), `
		INSERT INTO auth.account_permissions (tenant_id, account_id, permission_id, granted)
		SELECT ?, ?, id, TRUE
		FROM auth.permissions
		WHERE name = 'config:manage'
	`, tenantID, account.ID)
	require.NoError(t, err)
	rows, err := result.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 1, rows, "the seeded config:manage permission must exist")

	err = testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		var resolveErr error
		access, found, resolveErr = ResolveAccess(ctx, tx, tenantID, account.ID)
		return resolveErr
	})
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, access.GeneralRequests)
	require.True(t, access.EnrollmentRequests, "enrollment managers receive only enrollment requests")

	adminRole := testpkg.GetOrCreateTestRole(t, db, "admin")
	_, err = db.ExecContext(testpkg.Ctx(t), `
		INSERT INTO auth.account_roles (tenant_id, account_id, role_id)
		VALUES (?, ?, ?)
	`, tenantID, account.ID, adminRole.ID)
	require.NoError(t, err)

	err = testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		var resolveErr error
		access, found, resolveErr = ResolveAccess(ctx, tx, tenantID, account.ID)
		return resolveErr
	})
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, access.GeneralRequests)
	require.NotEmpty(t, access.SchoolName)
	require.NotEmpty(t, access.Subdomain)

	_, err = db.ExecContext(testpkg.Ctx(t), `
		UPDATE auth.account_tenants
		SET status = 'inactive', deactivated_at = NOW()
		WHERE tenant_id = ? AND account_id = ?
	`, tenantID, account.ID)
	require.NoError(t, err)

	err = testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		_, found, err = ResolveAccess(ctx, tx, tenantID, account.ID)
		return err
	})
	require.NoError(t, err)
	require.False(t, found)
}

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
