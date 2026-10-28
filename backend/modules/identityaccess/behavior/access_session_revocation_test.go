package behavior_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// accessSessionOf reads the family an access token rests on, as the
// per-request check sees it (#5194).
func accessSessionOf(t *testing.T, accessToken string) identityaccess.AccessSession {
	t.Helper()
	claims := decodePreviewTokenPayload(t, accessToken)
	session := identityaccess.AccessSession{FamilyID: fmt.Sprint(claims["family_id"])}
	if id, ok := claims["id"].(float64); ok {
		session.AccountID = int64(id)
	}
	if admin, ok := claims["acting_admin_id"].(float64); ok {
		session.ActingAdminID = int64(admin)
	}
	if scope, ok := claims["scope"].(string); ok {
		session.Scope = scope
	}
	if claims["family_id"] == nil {
		session.FamilyID = ""
	}
	return session
}

func familyRowCount(t *testing.T, db *bun.DB, familyID string) int64 {
	t.Helper()
	count, err := db.NewSelect().TableExpr("auth.tokens").Where("family_id = ?", familyID).Count(context.Background())
	require.NoError(t, err)
	return count
}

// TestLogoutEndsAccessTokensOnEveryInstance pins #5194: logout revokes the
// family, and a still-valid access token of that family is refused at once,
// also by a second, separately composed instance over the same database. The
// other device's family and earlier generations of a live family stay valid.
func TestLogoutEndsAccessTokensOnEveryInstance(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	firstInstance, secondInstance := setupAuthFactory(t, db), setupAuthFactory(t, db)
	service := newFixtureAuthService(t, db, firstInstance)
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials("access-revocation")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	access, refresh, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	otherDeviceAccess, _, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	session, otherDevice := accessSessionOf(t, access), accessSessionOf(t, otherDeviceAccess)
	require.NotEmpty(t, session.FamilyID)
	require.NotEqual(t, session.FamilyID, otherDevice.FamilyID)

	rotatedAccess, rotatedRefresh, err := service.RefreshToken(ctx, refresh)
	require.NoError(t, err)
	for _, instance := range []identityaccess.Owner{firstInstance, secondInstance} {
		require.NoError(t, instance.VerifyAccessSession(ctx, session), "the first generation's token lives while its family lives")
		require.NoError(t, instance.VerifyAccessSession(ctx, accessSessionOf(t, rotatedAccess)))
	}

	require.NoError(t, firstInstance.LogoutWithAudit(ctx, rotatedRefresh, "", ""))

	for name, instance := range map[string]identityaccess.Owner{"first": firstInstance, "second": secondInstance} {
		require.ErrorIs(t, instance.VerifyAccessSession(ctx, session), identityaccess.ErrInvalidToken, "%s instance", name)
		require.ErrorIs(t, instance.VerifyAccessSession(ctx, accessSessionOf(t, rotatedAccess)), identityaccess.ErrInvalidToken, "%s instance", name)
		require.NoError(t, instance.VerifyAccessSession(ctx, otherDevice), "%s instance keeps the other device", name)
	}
	_, _, err = service.RefreshToken(ctx, refresh)
	require.Error(t, err, "the late cookie's refresh token cannot renew the family either")
}

// TestVerifyAccessSessionRefusesForeignAndMissingFamilies covers the token
// shapes the check must refuse without a live family of the same account.
func TestVerifyAccessSessionRefusesForeignAndMissingFamilies(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := setupAuthFactory(t, db)
	service := newFixtureAuthService(t, db, module)
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials("access-foreign")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	access, _, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	session := accessSessionOf(t, access)

	withoutFamily := session
	withoutFamily.FamilyID = ""
	require.ErrorIs(t, module.VerifyAccessSession(ctx, withoutFamily), identityaccess.ErrInvalidToken)

	unknown := session
	unknown.FamilyID = "never-minted"
	require.ErrorIs(t, module.VerifyAccessSession(ctx, unknown), identityaccess.ErrInvalidToken)

	foreignAccount := session
	foreignAccount.AccountID = account.ID + 1
	require.ErrorIs(t, module.VerifyAccessSession(ctx, foreignAccount), identityaccess.ErrInvalidToken)

	otherPortal := session
	otherPortal.Scope = "parent"
	require.ErrorIs(t, module.VerifyAccessSession(ctx, otherPortal), identityaccess.ErrInvalidToken)

	_, err = db.ExecContext(ctx, `UPDATE auth.tokens SET expiry = NOW() - INTERVAL '1 second' WHERE family_id = ?`, session.FamilyID)
	require.NoError(t, err)
	require.ErrorIs(t, module.VerifyAccessSession(ctx, session), identityaccess.ErrInvalidToken, "an expired family ends its access tokens")
}

// TestRefreshRejectsSessionWithoutFamily pins the decision for sessions from
// before refresh families (#5194): their access tokens cannot be checked per
// request, so their refresh is refused, the row deleted, and the person
// signs in again.
func TestRefreshRejectsSessionWithoutFamily(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials("refresh-no-family")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	_, refresh, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE auth.tokens SET family_id = NULL WHERE account_id = ?`, account.ID)
	require.NoError(t, err)

	_, _, err = service.RefreshToken(ctx, refresh)
	require.ErrorIs(t, err, identityaccess.ErrInvalidToken)

	remaining, err := db.NewSelect().TableExpr("auth.tokens").Where("account_id = ?", account.ID).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, remaining, "the session without family is deleted")
}

// TestSwitchTenantRefusesRevokedSourceFamily pins that a switch cannot mint
// a new session out of a family logout already revoked (#5194).
func TestSwitchTenantRefusesRevokedSourceFamily(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials("switch-revoked")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	targetTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, targetTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, targetTenantID)
	access, refresh, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	family := accessSessionOf(t, access).FamilyID
	require.NoError(t, service.LogoutWithAudit(ctx, refresh, "", ""))

	_, _, err = service.SwitchTenant(ctx, account.ID, fmt.Sprintf("t%d", targetTenantID), family)
	require.ErrorIs(t, err, identityaccess.ErrInvalidToken)
	assert.Empty(t, liveTokens(t, db, account.ID), "no new session out of the revoked family")
}

// TestStaffPreviewRestsOnTheAdminsFamily pins that the access-only preview
// token dies with the admin session it was started from (#5194).
func TestStaffPreviewRestsOnTheAdminsFamily(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := setupAuthFactory(t, db)
	service := newFixtureAuthService(t, db, module)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	admin := testpkg.CreateTestAccount(t, db, "preview-family-admin")
	testpkg.EnsureAccountTenant(t, db, admin.ID, tenantID)
	assignSeededRoleForTenant(t, db, admin.ID, tenantID, "admin")
	_, target := testpkg.CreateTestCalendarStaff(t, db, "Erika", "Familie")
	adminAccess, adminRefresh, err := service.IssueTokensForAuthenticatedAccount(ctx, admin.ID, tenantID, "", "")
	require.NoError(t, err)
	adminSession := accessSessionOf(t, adminAccess)

	preview, err := service.StartStaffPreview(ctx, admin.ID, tenantID, target.ID, adminSession.FamilyID, "", "127.0.0.1", "go-test")
	require.NoError(t, err)
	previewSession := accessSessionOf(t, preview.AccessToken)
	assert.Equal(t, adminSession.FamilyID, previewSession.FamilyID)
	assert.Equal(t, target.ID, previewSession.AccountID)
	assert.Equal(t, admin.ID, previewSession.ActingAdminID)
	require.NoError(t, module.VerifyAccessSession(ctx, previewSession))

	require.NoError(t, service.LogoutWithAudit(ctx, adminRefresh, "", ""))
	require.ErrorIs(t, module.VerifyAccessSession(ctx, previewSession), identityaccess.ErrInvalidToken)
}

// TestLogoutWaitsForAConcurrentRotation pins the refresh/logout race of
// #5194 deterministically. The open transaction stands in for a refresh
// rotation: it holds the account lock refresh takes first and writes the
// successor generation. Logout must wait for that lock and delete the
// successor with the family; without the lock its DELETE runs on a snapshot
// without the successor, which then survives as a fresh, valid session.
func TestLogoutWaitsForAConcurrentRotation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials("logout-race")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	access, refresh, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	family := accessSessionOf(t, access).FamilyID

	rotation, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = rotation.Rollback() }()
	var rotationPID int
	require.NoError(t, rotation.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&rotationPID))
	_, err = rotation.ExecContext(ctx, `SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, account.ID)
	require.NoError(t, err)
	raw := make([]byte, 32)
	_, err = rand.Read(raw)
	require.NoError(t, err)
	_, err = rotation.ExecContext(ctx, `
		INSERT INTO auth.tokens (tenant_id, account_id, token, expiry, mobile, family_id, generation, portal_scope, created_at, updated_at)
		SELECT tenant_id, account_id, ?, NOW() + INTERVAL '1 hour', mobile, family_id, generation + 1, portal_scope, NOW(), NOW()
		FROM auth.tokens WHERE family_id = ? ORDER BY generation DESC LIMIT 1`, testpkg.TokenHash(t, db, hex.EncodeToString(raw)), family)
	require.NoError(t, err)

	logout := make(chan error, 1)
	go func() { logout <- service.LogoutWithAudit(context.WithoutCancel(ctx), refresh, "", "") }()
	require.Eventually(t, func() bool {
		var blocked int
		err := db.QueryRowContext(ctx, `SELECT count(*) FROM pg_stat_activity WHERE ? = ANY(pg_blocking_pids(pid))`, rotationPID).Scan(&blocked)
		return err == nil && blocked > 0
	}, 5*time.Second, 10*time.Millisecond, "logout must wait for the account lock of the running rotation")
	require.NoError(t, rotation.Commit())

	select {
	case err := <-logout:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("logout did not finish after the rotation committed")
	}
	assert.Zero(t, familyRowCount(t, db, family), "the successor committed during logout is revoked with its family")
}

// TestVerifyAccessSessionQueryBudget pins the per-request cost of the family
// check (#5194): one administrative read transaction with one statement,
// whatever the age of the family.
func TestVerifyAccessSessionQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := setupAuthFactory(t, db)
	service := newFixtureAuthService(t, db, module)
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials("access-budget")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	access, refresh, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	session := accessSessionOf(t, access)

	counter := testpkg.CaptureQueriesForContext(t, db)
	run := func() []string {
		counter.Reset()
		require.NoError(t, module.VerifyAccessSession(counter.Context(context.Background()), session))
		return counter.Queries()
	}
	fresh := run()
	for range 3 {
		_, refresh, err = service.RefreshToken(ctx, refresh)
		require.NoError(t, err)
	}
	rotated := run()
	assert.Len(t, rotated, len(fresh), "the check must not grow with the generations of the family")
	testpkg.AssertQueryBudget(t, "identity.access_session.verify", rotated)
}
