package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func newGuardianAccess(t *testing.T, db *bun.DB) identityaccess.GuardianAccess {
	t.Helper()
	module, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(identityCompose.Observation) {}})
	require.NoError(t, err)
	return module
}

func TestGuardianAccessObservationsCountOnlyWrittenRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "guardian-observations")
	var observations []identityCompose.Observation
	access, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(observation identityCompose.Observation) {
		observations = append(observations, observation)
	}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	_, err = access.FindAccount(ctx, account.ID)
	require.NoError(t, err)
	_, err = access.FindAccountByEmail(ctx, account.Email)
	require.NoError(t, err)
	first, err := access.GrantGuardianTenantAccess(ctx, account.ID)
	require.NoError(t, err)
	_, err = access.GrantGuardianTenantAccess(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, observations, 4)
	require.Zero(t, observations[0].Stats.Rows, "account SELECT changes no row")
	require.Zero(t, observations[1].Stats.Rows, "email SELECT changes no row")
	var expectedFirstWrites int64 = 1
	if first.RoleAssigned {
		expectedFirstWrites++
	}
	require.Equal(t, expectedFirstWrites, observations[2].Stats.Rows, "only mapping and role writes count")
	require.EqualValues(t, 1, observations[3].Stats.Rows, "repeat grant only updates the mapping")
}

type tenantAccessRow struct {
	Status        string
	DeactivatedAt *time.Time
	ActivatedAt   *time.Time
}

func tenantMapping(t *testing.T, db *bun.DB, accountID, tenantID int64) (tenantAccessRow, bool) {
	t.Helper()
	var row tenantAccessRow
	var count int
	require.NoError(t, db.NewRaw("SELECT count(*) FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?", accountID, tenantID).Scan(context.Background(), &count))
	if count == 0 {
		return row, false
	}
	require.NoError(t, db.NewRaw("SELECT status, deactivated_at, activated_at FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?", accountID, tenantID).Scan(context.Background(), &row.Status, &row.DeactivatedAt, &row.ActivatedAt))
	return row, true
}

func guardianRoleAssignments(t *testing.T, db *bun.DB, accountID, tenantID int64) []int64 {
	t.Helper()
	var roleIDs []int64
	require.NoError(t, db.NewRaw(`SELECT ar.role_id FROM auth.account_roles ar JOIN auth.roles r ON r.id = ar.role_id
		WHERE ar.account_id = ? AND ar.tenant_id = ? AND LOWER(r.name) = 'guardian' ORDER BY ar.id`, accountID, tenantID).Scan(context.Background(), &roleIDs))
	return roleIDs
}

// One platform account, two schools: granting guardian access in each tenant
// creates that tenant's mapping and role assignment only, the grant is
// idempotent, and a tenant-specific guardian role wins over the system role
// exactly like the legacy repositories resolved it.
func TestGuardianTenantAccessIsTenantScoped(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newGuardianAccess(t, db)
	account := testpkg.CreateTestAccount(t, db, "guardian-access")
	firstTenant := testpkg.Tenant(t)

	var secondTenant, tenantRoleID int64
	t.Run("second school", func(t *testing.T) {
		secondTenant = testpkg.OwnTenant(t)
		ctx := testpkg.Ctx(t)
		// A tenant-specific "guardian" role must be preferred over the system
		// role of the same name.
		require.NoError(t, db.NewRaw(`INSERT INTO auth.roles (name, description, is_system, tenant_id, base_role, created_at, updated_at)
			VALUES ('guardian', 'tenant guardian role', FALSE, ?, 'user', NOW(), NOW()) RETURNING id`, secondTenant).Scan(context.Background(), &tenantRoleID))
		_, mapped := tenantMapping(t, db, account.ID, secondTenant)
		require.False(t, mapped, "the account starts without a mapping to the second school")

		granted, err := access.GrantGuardianTenantAccess(ctx, account.ID)
		require.NoError(t, err)
		require.Equal(t, account.ID, granted.AccountID)
		require.Equal(t, secondTenant, granted.TenantID)
		require.Equal(t, tenantRoleID, granted.RoleID, "the tenant-specific guardian role is preferred")
		require.True(t, granted.RoleAssigned)

		again, err := access.GrantGuardianTenantAccess(ctx, account.ID)
		require.NoError(t, err)
		require.False(t, again.RoleAssigned, "a repeated grant must not assign the role twice")
		require.Equal(t, []int64{tenantRoleID}, guardianRoleAssignments(t, db, account.ID, secondTenant))
	})

	firstCtx := testpkg.ContextForTenant(testpkg.Ctx(t), firstTenant)
	granted, err := access.GrantGuardianTenantAccess(firstCtx, account.ID)
	require.NoError(t, err)
	require.Equal(t, firstTenant, granted.TenantID)
	require.NotEqual(t, tenantRoleID, granted.RoleID, "the first school must not see the second school's role")
	require.True(t, granted.RoleAssigned)

	for _, tenantID := range []int64{firstTenant, secondTenant} {
		mapping, mapped := tenantMapping(t, db, account.ID, tenantID)
		require.True(t, mapped)
		require.Equal(t, "active", mapping.Status)
		require.Nil(t, mapping.DeactivatedAt)
		require.Len(t, guardianRoleAssignments(t, db, account.ID, tenantID), 1, "exactly one guardian assignment per school")
	}

	// Accounts are platform-wide: both schools resolve the same account by
	// email, case-insensitively, and by ID.
	for _, tenantID := range []int64{firstTenant, secondTenant} {
		ctx := testpkg.ContextForTenant(testpkg.Ctx(t), tenantID)
		found, err := access.FindAccountByEmail(ctx, strings.ToUpper(account.Email))
		require.NoError(t, err)
		require.Equal(t, account.ID, found.ID)
		require.Equal(t, account.Email, found.Email)
		byID, err := access.FindAccount(ctx, account.ID)
		require.NoError(t, err)
		require.Equal(t, account.Email, byID.Email)
	}
}

// An offboarded guardian keeps the mapping row with status inactive. A new
// approval must reactivate it; a create-if-missing write would leave the
// parent locked out of the school that just accepted the child.
func TestGuardianTenantAccessReactivatesInactiveMapping(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newGuardianAccess(t, db)
	account := testpkg.CreateTestAccount(t, db, "reactivated-guardian")
	tenantID := testpkg.Tenant(t)
	_, err := db.NewRaw("UPDATE auth.account_tenants SET status = 'inactive', deactivated_at = NOW() WHERE account_id = ? AND tenant_id = ?", account.ID, tenantID).Exec(context.Background())
	require.NoError(t, err)

	granted, err := access.GrantGuardianTenantAccess(testpkg.Ctx(t), account.ID)
	require.NoError(t, err)
	require.True(t, granted.RoleAssigned)

	mapping, mapped := tenantMapping(t, db, account.ID, tenantID)
	require.True(t, mapped)
	require.Equal(t, "active", mapping.Status)
	require.Nil(t, mapping.DeactivatedAt)
	require.NotNil(t, mapping.ActivatedAt)
}

// A mapping the approval creates is not an invitation: invited_at stays NULL
// like the legacy model wrote it, so invitation projections keep falling back
// to created_at.
func TestGuardianTenantAccessCreatesMappingWithoutInvitation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newGuardianAccess(t, db)
	account := testpkg.CreateTestAccount(t, db, "uninvited-guardian")
	tenantID := testpkg.Tenant(t)
	_, err := db.NewRaw("DELETE FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?", account.ID, tenantID).Exec(context.Background())
	require.NoError(t, err)
	testpkg.OwnTestAccount(t, db, account.ID)

	granted, err := access.GrantGuardianTenantAccess(testpkg.Ctx(t), account.ID)
	require.NoError(t, err)
	require.True(t, granted.RoleAssigned)
	var invitedAt *time.Time
	require.NoError(t, db.NewRaw("SELECT invited_at FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?", account.ID, tenantID).Scan(context.Background(), &invitedAt))
	require.Nil(t, invitedAt)
}

// Grants join the caller's tenant transaction: a failure after the mapping
// write rolls the mapping back with the caller's work.
func TestGuardianTenantAccessJoinsCallerTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newGuardianAccess(t, db)
	account := testpkg.CreateTestAccount(t, db, "rolled-back-guardian")
	tenantID := testpkg.Tenant(t)
	_, err := db.NewRaw("DELETE FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?", account.ID, tenantID).Exec(context.Background())
	require.NoError(t, err)
	testpkg.OwnTestAccount(t, db, account.ID)

	rollback := context.Canceled
	err = testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		granted, grantErr := access.GrantGuardianTenantAccess(txCtx, account.ID)
		require.NoError(t, grantErr)
		require.True(t, granted.RoleAssigned)
		_, mapped := tenantMapping(t, db, account.ID, tenantID)
		require.False(t, mapped, "the mapping must not be visible outside the caller's transaction")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	_, mapped := tenantMapping(t, db, account.ID, tenantID)
	require.False(t, mapped, "the caller's rollback must discard the grant")
	require.Empty(t, guardianRoleAssignments(t, db, account.ID, tenantID))
}

// Every read and write surfaces a driver failure with its stable operation
// prefix, a missing tenant refuses the grant, and unknown accounts are the
// sentinel error rather than an empty value.
func TestGuardianAccessPreservesFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newGuardianAccess(t, db)
	account := testpkg.CreateTestAccount(t, db, "failing-guardian")
	ctx := testpkg.Ctx(t)

	_, err := access.GrantGuardianTenantAccess(context.Background(), account.ID)
	require.ErrorIs(t, err, identityaccess.ErrTenantRequired)
	_, err = access.GrantGuardianTenantAccess(ctx, 0)
	require.ErrorIs(t, err, identityaccess.ErrAccountNotFound)

	var missingID int64
	require.NoError(t, db.NewRaw("SELECT nextval(pg_get_serial_sequence('auth.accounts', 'id'))").Scan(context.Background(), &missingID))
	_, err = access.FindAccount(ctx, missingID)
	require.ErrorIs(t, err, identityaccess.ErrAccountNotFound)
	_, err = access.FindAccountByEmail(ctx, "nobody-"+t.Name()+"@example.test")
	require.ErrorIs(t, err, identityaccess.ErrAccountNotFound)
	_, err = access.FindAccountByEmail(ctx, "   ")
	require.ErrorIs(t, err, identityaccess.ErrAccountNotFound)

	operations := []struct {
		name   string
		prefix string
		run    func(context.Context) error
	}{
		{"find account", "identity access: find account", func(ctx context.Context) error {
			_, err := access.FindAccount(ctx, account.ID)
			return err
		}},
		{"find account by email", "identity access: find account by email", func(ctx context.Context) error {
			_, err := access.FindAccountByEmail(ctx, account.Email)
			return err
		}},
		{"grant guardian tenant access", "identity access: grant guardian tenant access", func(ctx context.Context) error {
			_, err := access.GrantGuardianTenantAccess(ctx, account.ID)
			return err
		}},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			// A cancelled statement is the cheapest real driver failure; each
			// operation runs in its own transaction because the failure
			// poisons it.
			err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
				cancelled, cancel := context.WithCancel(txCtx)
				cancel()
				return operation.run(cancelled)
			})
			require.ErrorIs(t, err, context.Canceled)
			require.ErrorContains(t, err, operation.prefix)
		})
	}

	// The failures left the healthy path intact.
	granted, err := access.GrantGuardianTenantAccess(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, testpkg.Tenant(t), granted.TenantID)
	found, err := access.FindAccountByEmail(ctx, account.Email)
	require.NoError(t, err)
	require.Equal(t, account.ID, found.ID)
}
