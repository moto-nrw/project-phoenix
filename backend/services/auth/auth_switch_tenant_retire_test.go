package auth_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/rotation"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type liveTokenRow struct {
	FamilyID        string     `bun:"family_id"`
	Expiry          time.Time  `bun:"expiry"`
	FamilyExpiryCap *time.Time `bun:"family_expiry_cap"`
}

// liveTokens returns the account's unrotated, unexpired refresh tokens, newest first.
func liveTokens(t *testing.T, db *bun.DB, accountID int64) []liveTokenRow {
	t.Helper()
	var rows []liveTokenRow
	require.NoError(t, db.NewSelect().
		ColumnExpr("family_id, expiry, family_expiry_cap").
		TableExpr("auth.tokens").
		Where("account_id = ?", accountID).
		Where("rotated_at IS NULL").
		Where("expiry > NOW()").
		OrderExpr("id DESC").
		Scan(context.Background(), &rows))
	return rows
}

// TestSwitchTenantRetiredFamilyRefreshKeepsDeadline pins the stale-request
// path: a refresh of the retired family must not grant its successor a new
// full refresh lifetime.
func TestSwitchTenantRetiredFamilyRefreshKeepsDeadline(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials("switch-retire-refresh")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	targetTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, targetTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, targetTenantID)

	_, presentedRefresh, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	presentedFamily := tokenFamilyID(t, db, account.ID, 0)
	_, _, err = service.SwitchTenant(ctx, account.ID, fmt.Sprintf("t%d", targetTenantID), presentedFamily)
	require.NoError(t, err)

	_, refreshedToken, err := service.RefreshToken(ctx, presentedRefresh)
	require.NoError(t, err)
	_, _, err = service.RefreshToken(ctx, refreshedToken)
	require.NoError(t, err)

	var retired liveTokenRow
	for _, row := range liveTokens(t, db, account.ID) {
		if row.FamilyID == presentedFamily {
			retired = row
			break
		}
	}
	require.NotNil(t, retired.FamilyExpiryCap)
	require.WithinDuration(t, *retired.FamilyExpiryCap, retired.Expiry, time.Second)
	require.False(t, retired.Expiry.After(time.Now().Add(rotation.RecoveryGrace)), "refresh successors stay within the retirement grace")
}

// TestSwitchTenantRetiresPresentedFamily pins #2952: the family behind the
// caller's access token gets a bounded grace period instead of the full
// refresh lifetime, so switch-minted families stop piling up against the
// session cap. The successor keeps the full lifetime.
func TestSwitchTenantRetiresPresentedFamily(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials("switch-retire")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	targetTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, targetTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, targetTenantID)

	_, presentedRefresh, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	presentedFamily := tokenFamilyID(t, db, account.ID, 0)

	before := time.Now()
	_, _, err = service.SwitchTenant(ctx, account.ID, fmt.Sprintf("t%d", targetTenantID), presentedFamily)
	require.NoError(t, err)

	rows := liveTokens(t, db, account.ID)
	require.Len(t, rows, 2, "the presented family stays alive for in-flight requests")
	require.NotEqual(t, presentedFamily, rows[0].FamilyID)
	require.True(t, rows[0].Expiry.After(before.Add(time.Hour)), "successor keeps the full refresh lifetime")
	require.Equal(t, presentedFamily, rows[1].FamilyID)
	require.True(t, rows[1].Expiry.After(before), "retired family is not dead yet")
	require.False(t, rows[1].Expiry.After(time.Now().Add(rotation.RecoveryGrace)), "retired family expires within the recovery grace")

	// Within the grace an in-flight request holding the old cookie still refreshes.
	_, _, err = service.RefreshToken(ctx, presentedRefresh)
	require.NoError(t, err)
}

// TestSwitchTenantPingPongKeepsOtherDeviceSession is the production shape
// behind #2952: two tabs on two schools auto-switch back and forth while a
// second device sits on its own session. The second device must survive.
func TestSwitchTenantPingPongKeepsOtherDeviceSession(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials("switch-pingpong")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	schoolA := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, schoolA)
	testpkg.MapAccountToTenant(t, db, account.ID, schoolA)
	schoolB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, schoolB)
	testpkg.MapAccountToTenant(t, db, account.ID, schoolB)

	// Other device: logs in once and stays put.
	_, otherDeviceRefresh, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	otherDeviceFamily := tokenFamilyID(t, db, account.ID, 0)

	// Browser with two tabs: logs in, then the tenant guard flips the shared
	// cookie between the two schools, each flip presenting the previous family.
	_, _, err = service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	presented := liveTokens(t, db, account.ID)[0].FamilyID
	for i := range 8 {
		target := schoolA
		if i%2 == 1 {
			target = schoolB
		}
		_, _, err = service.SwitchTenant(ctx, account.ID, fmt.Sprintf("t%d", target), presented)
		require.NoError(t, err, "switch %d", i)
		presented = liveTokens(t, db, account.ID)[0].FamilyID
	}

	var otherDeviceStillLive bool
	for _, row := range liveTokens(t, db, account.ID) {
		if row.FamilyID == otherDeviceFamily && row.Expiry.After(time.Now().Add(time.Hour)) {
			otherDeviceStillLive = true
		}
	}
	require.True(t, otherDeviceStillLive, "the session cap must not revoke the other device's family")
	_, _, err = service.RefreshToken(ctx, otherDeviceRefresh)
	require.NoError(t, err, "the other device keeps refreshing after the tab ping-pong")
}
