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
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
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

// TestRefreshGivesALegacyHeadAFamily pins the rotation side of #5194: a
// head from before refresh families that the backfill has not reached (it
// rotated while the migration ran) gets a family at its next refresh,
// written onto its predecessor too, so the person stays signed in, the new
// access token passes the family check and a logout revokes the chain.
func TestRefreshGivesALegacyHeadAFamily(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := setupAuthFactory(t, db)
	service := newFixtureAuthService(t, db, module)
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials("refresh-no-family")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	_, refresh, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE auth.tokens SET family_id = NULL WHERE account_id = ?`, account.ID)
	require.NoError(t, err)

	access, renewed, err := service.RefreshToken(ctx, refresh)
	require.NoError(t, err)

	session := accessSessionOf(t, access)
	require.NotEmpty(t, session.FamilyID)
	require.NoError(t, module.VerifyAccessSession(ctx, session))
	var families []string
	require.NoError(t, db.NewRaw(`SELECT DISTINCT COALESCE(family_id, '') FROM auth.tokens WHERE account_id = ?`, account.ID).Scan(ctx, &families))
	assert.Equal(t, []string{session.FamilyID}, families, "head and predecessor share the new family")

	require.NoError(t, module.LogoutWithAudit(ctx, renewed, "", ""))
	require.ErrorIs(t, module.VerifyAccessSession(ctx, session), identityaccess.ErrInvalidToken)
	assert.Zero(t, familyRowCount(t, db, session.FamilyID))
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
	querybudgets.AssertQueryBudget(t, "identity.access_session.verify", rotated)
}

// TestPortalLogoutEndsOnlyThatPortalsAccessTokens pins #5194 for the parent
// and school portals: their logout revokes their own family through the
// shared /auth/logout, the per-request check then refuses their access
// tokens, and the other portal's family of the same account stays valid.
func TestPortalLogoutEndsOnlyThatPortalsAccessTokens(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := setupAuthFactory(t, db)
	service := newFixtureAuthService(t, db, module)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("portal-logout")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	assignSeededRole(t, db, account.ID, tenantID, "user")
	assignSeededRole(t, db, account.ID, tenantID, "guardian")

	parentAccess, parentRefresh, err := service.LoginParent(ctx, email, testPassword)
	require.NoError(t, err)
	tenantAccess, _, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	parent, staff := accessSessionOf(t, parentAccess), accessSessionOf(t, tenantAccess)
	require.Equal(t, "parent", parent.Scope)
	require.NoError(t, module.VerifyAccessSession(ctx, parent))

	require.NoError(t, module.LogoutWithAudit(ctx, parentRefresh, "", ""))

	require.ErrorIs(t, module.VerifyAccessSession(ctx, parent), identityaccess.ErrInvalidToken, "the parent token ends with its family")
	require.NoError(t, module.VerifyAccessSession(ctx, staff), "the staff session of the same account stays valid")

	schoolTenant, _ := testpkg.CreateTestTenant(t, db)
	_, teacherID := createLehrkraftAccount(t, db, service, "portal-logout-school", schoolTenant)
	schoolAccess, schoolRefresh, err := service.IssueSchoolTokensForAuthenticatedAccount(testpkg.TenantContext(schoolTenant), teacherID, schoolTenant, "", "")
	require.NoError(t, err)
	school := accessSessionOf(t, schoolAccess)
	require.Equal(t, "school", school.Scope)
	require.NoError(t, module.VerifyAccessSession(ctx, school))
	require.NoError(t, module.LogoutWithAudit(ctx, schoolRefresh, "", ""))
	require.ErrorIs(t, module.VerifyAccessSession(ctx, school), identityaccess.ErrInvalidToken, "the school token ends with its family")
}

// TestLogoutOfPartlyPlacedChainRevokesEverything pins the fail-closed rule of
// #5194: in a chain whose older rows carry no family (a backfill could not
// place them), a logout presenting any of its refresh tokens revokes the
// whole chain. No access token of the chain stays valid and no row survives.
func TestLogoutOfPartlyPlacedChainRevokesEverything(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := setupAuthFactory(t, db)
	service := newFixtureAuthService(t, db, module)
	ctx := testpkg.Ctx(t)

	for _, presented := range []string{"oldest predecessor", "head"} {
		t.Run(presented, func(t *testing.T) {
			email, username := uniqueTestCredentials("partly-placed")
			account, err := service.Register(ctx, email, username, testPassword, nil, 0)
			require.NoError(t, err)
			testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
			_, refresh, err := service.Login(ctx, email, testPassword)
			require.NoError(t, err)
			tokens := []string{refresh}
			var access string
			for range 3 {
				access, refresh, err = service.RefreshToken(ctx, refresh)
				require.NoError(t, err)
				tokens = append(tokens, refresh)
			}
			session := accessSessionOf(t, access)
			// Only the head keeps its family; its rotated predecessors do not.
			_, err = db.ExecContext(ctx, `UPDATE auth.tokens SET family_id = NULL WHERE account_id = ? AND rotated_at IS NOT NULL`, account.ID)
			require.NoError(t, err)
			require.NoError(t, module.VerifyAccessSession(ctx, session))

			logoutToken := tokens[len(tokens)-1]
			if presented == "oldest predecessor" {
				logoutToken = tokens[0]
			}
			require.NoError(t, module.LogoutWithAudit(ctx, logoutToken, "", ""))

			require.ErrorIs(t, module.VerifyAccessSession(ctx, session), identityaccess.ErrInvalidToken, "no access token of the chain stays valid")
			remaining, err := db.NewSelect().TableExpr("auth.tokens").Where("account_id = ?", account.ID).Count(ctx)
			require.NoError(t, err)
			assert.Zero(t, remaining, "no row of the chain survives its logout")
		})
	}
}

// TestLogoutIgnoresAnotherFamilysCollidingToken pins that the chain walk of
// a logout follows only unique links (#5194): when another family of the
// same account holds a token hash that collides with the logged-out chain's
// hand-off, the logout revokes its own family completely and leaves the
// other one valid.
func TestLogoutIgnoresAnotherFamilysCollidingToken(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := setupAuthFactory(t, db)
	service := newFixtureAuthService(t, db, module)
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials("colliding-token")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	_, ownFirst, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	ownAccess, _, err := service.RefreshToken(ctx, ownFirst)
	require.NoError(t, err)
	otherAccess, _, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	own, other := accessSessionOf(t, ownAccess), accessSessionOf(t, otherAccess)

	// The other family's head now holds the token hash the own predecessor
	// hands off to.
	_, err = db.ExecContext(ctx, `
		UPDATE auth.tokens SET token = (
			SELECT replacement_token FROM auth.tokens WHERE family_id = ? AND rotated_at IS NOT NULL
		) WHERE family_id = ?`, own.FamilyID, other.FamilyID)
	require.NoError(t, err)

	require.NoError(t, module.LogoutWithAudit(ctx, ownFirst, "", ""))

	require.ErrorIs(t, module.VerifyAccessSession(ctx, own), identityaccess.ErrInvalidToken, "the own chain is revoked completely")
	require.NoError(t, module.VerifyAccessSession(ctx, other), "the other family stays valid")
	assert.Zero(t, familyRowCount(t, db, own.FamilyID))
	assert.Positive(t, familyRowCount(t, db, other.FamilyID), "the other family keeps its session")
}
