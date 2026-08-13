package auth_test

import (
	"context"
	"testing"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func assignSeededRole(t *testing.T, db *bun.DB, accountID, tenantID int64, roleName string) {
	t.Helper()
	ctx := testpkg.TenantContext(tenantID)
	var roleID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("name = ?", roleName).
		Scan(ctx, &roleID))
	assignment := &authModels.AccountRole{AccountID: accountID, RoleID: roleID}
	assignment.SetTenantID(tenantID)
	_, err := db.NewInsert().Model(assignment).ModelTableExpr(`auth.account_roles`).Exec(ctx)
	require.NoError(t, err)
}

func assertTokenCountByPortal(t *testing.T, db *bun.DB, accountID int64, portalScope string, want int) {
	t.Helper()
	count, err := db.NewSelect().
		Model((*authModels.Token)(nil)).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".account_id = ?`, accountID).
		Where(`"token".portal_scope = ?`, portalScope).
		Where(`"token".rotated_at IS NULL`).
		Where(`"token".expiry > NOW()`).
		Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, want, count, "active %s sessions", portalScope)
}

func TestLogoutLeavesOtherDeviceSessionsIntact(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("logout-other-device")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	_, firstRefresh, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	_, secondRefresh, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)

	require.NoError(t, service.LogoutWithAudit(ctx, firstRefresh, "", ""))

	_, _, err = service.RefreshToken(ctx, firstRefresh)
	require.Error(t, err, "the logged-out device must not be able to refresh")

	_, _, err = service.RefreshToken(ctx, secondRefresh)
	require.NoError(t, err, "a second device session must survive logout of the first")
}

func TestLogoutLeavesStaffPushOnOtherDevices(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("logout-other-push")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	_, firstRefresh, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	_, secondRefresh, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)

	firstFamily := tokenFamilyID(t, db, account.ID, 0)
	secondFamily := tokenFamilyID(t, db, account.ID, 1)
	require.NotEqual(t, firstFamily, secondFamily)

	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/this-device", firstFamily)
	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/other-device", secondFamily)

	require.NoError(t, service.LogoutWithAudit(ctx, firstRefresh, "", ""))

	_, _, err = service.RefreshToken(ctx, firstRefresh)
	require.Error(t, err)
	_, _, err = service.RefreshToken(ctx, secondRefresh)
	require.NoError(t, err)

	require.Equal(t, 0, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/this-device"))
	require.Equal(t, 1, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/other-device"))
}

func tokenFamilyID(t *testing.T, db *bun.DB, accountID int64, offset int) string {
	t.Helper()
	var familyID string
	require.NoError(t, db.NewSelect().
		ColumnExpr("family_id").
		TableExpr("auth.tokens").
		Where("account_id = ?", accountID).
		Where("rotated_at IS NULL").
		OrderExpr("id ASC").
		Offset(offset).
		Limit(1).
		Scan(context.Background(), &familyID))
	require.NotEmpty(t, familyID)
	return familyID
}

func insertStaffPush(t *testing.T, db *bun.DB, accountID, tenantID int64, endpoint, familyID string) {
	t.Helper()
	sub := &iotModels.PushSubscription{
		AccountID:     accountID,
		Portal:        iotModels.PushPortalStaff,
		Endpoint:      endpoint,
		P256dh:        "p256dh-key",
		Auth:          "auth-key",
		TokenFamilyID: familyID,
	}
	sub.SetTenantID(tenantID)
	_, err := db.NewInsert().Model(sub).ModelTableExpr("iot.push_subscriptions").Exec(context.Background())
	require.NoError(t, err)
}

func TestRevokeAllTokensClearsStaffPushAcrossTenants(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("revoke-all-push")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	secondaryTenantID := account.ID + 1_000_000_000
	testpkg.EnsureTestTenant(t, db, secondaryTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, secondaryTenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/school-a", "family-a")
	insertStaffPush(t, db, account.ID, secondaryTenantID, "https://fcm.googleapis.com/school-b", "family-b")

	require.NoError(t, service.RevokeAllTokens(ctx, int(account.ID)))

	require.Equal(t, 0, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/school-a"))
	require.Equal(t, 0, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/school-b"))
}

func countStaffPush(t *testing.T, db *bun.DB, accountID int64, endpoint string) int {
	t.Helper()
	count, err := db.NewSelect().
		Model((*iotModels.PushSubscription)(nil)).
		ModelTableExpr(`iot.push_subscriptions AS "push_subscription"`).
		Where("account_id = ?", accountID).
		Where("portal = ?", iotModels.PushPortalStaff).
		Where("endpoint = ?", endpoint).
		Count(context.Background())
	require.NoError(t, err)
	return count
}

func TestLogoutLeavesOtherPortalSessionsIntact(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("logout-other-portal")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	assignSeededRole(t, db, account.ID, tenantID, authModels.BaseRoleUser)
	assignSeededRole(t, db, account.ID, tenantID, authModels.BaseRoleGuardian)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	_, tenantRefresh, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	_, parentRefresh, err := service.LoginParent(ctx, email, testPassword)
	require.NoError(t, err)

	require.NoError(t, service.LogoutWithAudit(ctx, tenantRefresh, "", ""))

	_, _, err = service.RefreshToken(ctx, tenantRefresh)
	require.Error(t, err, "the tenant session must be revoked")

	_, _, err = service.RefreshToken(ctx, parentRefresh)
	require.NoError(t, err, "a parent-portal session must survive tenant logout")
}

func TestSessionCapDoesNotEvictOtherPortalSessions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("session-cap-portal")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	assignSeededRole(t, db, account.ID, tenantID, authModels.BaseRoleUser)
	assignSeededRole(t, db, account.ID, tenantID, authModels.BaseRoleGuardian)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	_, parentRefresh, err := service.LoginParent(ctx, email, testPassword)
	require.NoError(t, err)

	for range 5 {
		_, _, err = service.Login(ctx, email, testPassword)
		require.NoError(t, err)
	}

	_, _, err = service.RefreshToken(ctx, parentRefresh)
	require.NoError(t, err, "five tenant sessions must not evict a parent-portal session")

	assertTokenCountByPortal(t, db, account.ID, authModels.PortalScopeParent, 1)
	assertTokenCountByPortal(t, db, account.ID, authModels.PortalScopeTenant, 5)
}
