package behavior_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// switchLineageFixture is an account signed in at its home school with a
// mapping at a second school it can switch to.
type switchLineageFixture struct {
	db             *bun.DB
	module         identityaccess.Owner
	service        testAuthService
	accountID      int64
	homeTenantID   int64
	targetTenantID int64
	access         string
	refresh        string
}

func newSwitchLineageFixture(t *testing.T, prefix string) switchLineageFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module := setupAuthFactory(t, db)
	service := newFixtureAuthService(t, db, module)
	homeTenantID := testpkg.Tenant(t)
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials(prefix)
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, homeTenantID)
	targetTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, targetTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, targetTenantID)
	access, refresh, err := service.IssueTokensForAuthenticatedAccount(ctx, account.ID, homeTenantID, "", "")
	require.NoError(t, err)
	return switchLineageFixture{
		db: db, module: module, service: service, accountID: account.ID,
		homeTenantID: homeTenantID, targetTenantID: targetTenantID, access: access, refresh: refresh,
	}
}

func (f switchLineageFixture) switchToTarget(ctx context.Context, sourceFamilyID string) (string, string, error) {
	return f.service.SwitchTenant(ctx, f.accountID, fmt.Sprintf("t%d", f.targetTenantID), sourceFamilyID)
}

// TestLogoutRevokesAFamilySwitchedFromIt pins #5422: a school switch that
// committed before the logout of its source family leaves no live family.
// Its answer may reach the browser after the logout deleted the cookie;
// the family it names must already be gone, so neither the access token,
// the refresh token nor the cookie gate's family check keeps it alive.
func TestLogoutRevokesAFamilySwitchedFromIt(t *testing.T) {
	t.Parallel()
	f := newSwitchLineageFixture(t, "logout-switched")
	ctx := testpkg.Ctx(t)
	source := accessSessionOf(t, f.access)
	switchedAccess, switchedRefresh, err := f.switchToTarget(ctx, source.FamilyID)
	require.NoError(t, err)
	switched := accessSessionOf(t, switchedAccess)
	require.NotEqual(t, source.FamilyID, switched.FamilyID, "a switch mints a family of its own")
	require.NoError(t, f.module.VerifyAccessSession(ctx, switched))

	require.NoError(t, f.service.LogoutWithAudit(ctx, f.refresh, "", ""))

	require.ErrorIs(t, f.module.VerifyAccessSession(ctx, switched), identityaccess.ErrInvalidToken, "the switched family ends with the logout")
	assert.Zero(t, familyRowCount(t, f.db, switched.FamilyID))
	_, _, err = f.service.RefreshToken(ctx, switchedRefresh)
	require.Error(t, err, "the switched family's refresh token is worthless")
	assert.Empty(t, liveTokens(t, f.db, f.accountID), "no family of the lineage survives")
}

// TestLogoutOfSwitchedFamilyRevokesItsRetiredSource pins the other
// direction: the retired source family lives on through its grace, and a
// logout of the family switched to must not leave it usable.
func TestLogoutOfSwitchedFamilyRevokesItsRetiredSource(t *testing.T) {
	t.Parallel()
	f := newSwitchLineageFixture(t, "logout-source")
	ctx := testpkg.Ctx(t)
	source := accessSessionOf(t, f.access)
	_, switchedRefresh, err := f.switchToTarget(ctx, source.FamilyID)
	require.NoError(t, err)
	require.NoError(t, f.module.VerifyAccessSession(ctx, source), "the retired source lives through its grace")

	require.NoError(t, f.service.LogoutWithAudit(ctx, switchedRefresh, "", ""))

	require.ErrorIs(t, f.module.VerifyAccessSession(ctx, source), identityaccess.ErrInvalidToken)
	assert.Zero(t, familyRowCount(t, f.db, source.FamilyID))
	assert.Empty(t, liveTokens(t, f.db, f.accountID))
}

// TestLogoutRevokesSwitchesAcrossTheWholeLineage covers a stale tab that
// switches from the retired source again: every family that descends from
// the same login ends, whichever of them is logged out.
func TestLogoutRevokesSwitchesAcrossTheWholeLineage(t *testing.T) {
	t.Parallel()
	f := newSwitchLineageFixture(t, "logout-lineage")
	ctx := testpkg.Ctx(t)
	source := accessSessionOf(t, f.access)
	firstAccess, firstRefresh, err := f.switchToTarget(ctx, source.FamilyID)
	require.NoError(t, err)
	secondAccess, _, err := f.switchToTarget(ctx, source.FamilyID)
	require.NoError(t, err, "the retired source still switches within its grace")
	thirdAccess, _, err := f.service.SwitchTenant(ctx, f.accountID, fmt.Sprintf("t%d", f.homeTenantID), accessSessionOf(t, secondAccess).FamilyID)
	require.NoError(t, err)

	other := newOtherDeviceSession(t, f)

	require.NoError(t, f.service.LogoutWithAudit(ctx, firstRefresh, "", ""))

	for name, access := range map[string]string{"source": f.access, "first": firstAccess, "second": secondAccess, "third": thirdAccess} {
		require.ErrorIs(t, f.module.VerifyAccessSession(ctx, accessSessionOf(t, access)), identityaccess.ErrInvalidToken, name)
	}
	require.NoError(t, f.module.VerifyAccessSession(ctx, other), "another login of the account keeps its session")
}

func newOtherDeviceSession(t *testing.T, f switchLineageFixture) identityaccess.AccessSession {
	t.Helper()
	access, _, err := f.service.IssueTokensForAuthenticatedAccount(testpkg.Ctx(t), f.accountID, f.homeTenantID, "", "")
	require.NoError(t, err)
	return accessSessionOf(t, access)
}

// TestLogoutWaitsForAConcurrentSwitch drives the #5422 race with both real
// commands. An open transaction holds the account lock both take; the
// switch queues on it first, the logout behind it. Released, the switch
// commits its family and the logout, granted the lock next, must read and
// revoke it. A logout that read the account's sessions before the switch
// committed would leave the switched family alive.
func TestLogoutWaitsForAConcurrentSwitch(t *testing.T) {
	t.Parallel()
	f := newSwitchLineageFixture(t, "logout-switch-race")
	ctx := testpkg.Ctx(t)
	source := accessSessionOf(t, f.access)

	holder, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	var holderPID int
	require.NoError(t, holder.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID))
	_, err = holder.ExecContext(ctx, `SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, f.accountID)
	require.NoError(t, err)

	type switchResult struct {
		access string
		err    error
	}
	switched := make(chan switchResult, 1)
	go func() {
		access, _, err := f.switchToTarget(context.WithoutCancel(ctx), source.FamilyID)
		switched <- switchResult{access: access, err: err}
	}()
	switchPID := waitForBlockedBackend(t, f.db, []int{holderPID}, "the switch must wait for the account lock")

	logout := make(chan error, 1)
	go func() { logout <- f.service.LogoutWithAudit(context.WithoutCancel(ctx), f.refresh, "", "") }()
	waitForBlockedBackend(t, f.db, []int{holderPID, switchPID}, "the logout must queue behind the switch")
	require.NoError(t, holder.Commit())

	var result switchResult
	select {
	case result = <-switched:
	case <-time.After(10 * time.Second):
		t.Fatal("switch did not finish after the lock was released")
	}
	select {
	case err := <-logout:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("logout did not finish after the switch committed")
	}
	require.NoError(t, result.err, "the switch held the lock first and committed")
	require.ErrorIs(t, f.module.VerifyAccessSession(ctx, accessSessionOf(t, result.access)), identityaccess.ErrInvalidToken,
		"the family the switch committed before the logout is revoked with it")
	assert.Empty(t, liveTokens(t, f.db, f.accountID))
}

// waitForBlockedBackend waits until a backend other than blockers waits on
// one of them and returns its pid.
func waitForBlockedBackend(t *testing.T, db *bun.DB, blockers []int, message string) int {
	t.Helper()
	var pid int
	require.Eventually(t, func() bool {
		err := db.QueryRowContext(context.Background(), `
			SELECT pid FROM pg_stat_activity
			WHERE NOT (pid = ANY(?::int[])) AND pg_blocking_pids(pid) && ?::int[]
			ORDER BY pid LIMIT 1`, intArray(blockers), intArray(blockers)).Scan(&pid)
		return err == nil && pid > 0
	}, 5*time.Second, 10*time.Millisecond, message)
	return pid
}

func intArray(values []int) string {
	parts := "{"
	for i, value := range values {
		if i > 0 {
			parts += ","
		}
		parts += fmt.Sprint(value)
	}
	return parts + "}"
}

// TestLogoutAuditsSwitchedFamiliesInTheirOwnSchool pins the tenant
// boundary of the lineage logout: each school's ledger records the
// revocation of its own session, and none records another school's.
func TestLogoutAuditsSwitchedFamiliesInTheirOwnSchool(t *testing.T) {
	t.Parallel()
	f := newSwitchLineageFixture(t, "logout-switch-audit")
	ctx := testpkg.Ctx(t)
	source := accessSessionOf(t, f.access)
	_, _, err := f.switchToTarget(ctx, source.FamilyID)
	require.NoError(t, err)

	require.NoError(t, f.service.LogoutWithAudit(ctx, f.refresh, "192.0.2.11", "lineage-test"))

	for _, tenantID := range []int64{f.homeTenantID, f.targetTenantID} {
		var events []authEventRow
		require.NoError(t, f.db.NewSelect().Model(&events).
			Where(`"auth_event".account_id = ?`, f.accountID).
			Where(`"auth_event".tenant_id = ?`, tenantID).
			Where(`"auth_event".event_type = ?`, authEventTokenRevoked).
			Scan(ctx))
		var logoutEvents int
		for _, event := range events {
			if event.Metadata["reason"] == "logout" {
				logoutEvents++
			}
		}
		assert.Equal(t, 1, logoutEvents, "school %d records the logout revocation of its own session once", tenantID)
	}
}
