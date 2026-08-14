package auth_test

import (
	"context"
	"testing"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	insertPush(t, db, accountID, tenantID, iotModels.PushPortalStaff, endpoint, familyID)
}

func insertParentPush(t *testing.T, db *bun.DB, accountID, tenantID int64, endpoint, familyID string) {
	t.Helper()
	insertPush(t, db, accountID, tenantID, iotModels.PushPortalParent, endpoint, familyID)
}

func insertPush(t *testing.T, db *bun.DB, accountID, tenantID int64, portal, endpoint, familyID string) {
	t.Helper()
	sub := &iotModels.PushSubscription{
		AccountID:     accountID,
		Portal:        portal,
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
		testpkg.CleanupTestTenant(t, db, tenantID, secondaryTenantID)
	})

	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/school-a", "family-a")
	insertStaffPush(t, db, account.ID, secondaryTenantID, "https://fcm.googleapis.com/school-b", "family-b")

	require.NoError(t, service.RevokeAllTokens(ctx, int(account.ID)))

	require.Equal(t, 0, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/school-a"))
	require.Equal(t, 0, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/school-b"))
}

func TestRevokeAllTokensFromTenantTxClearsOtherSchools(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("revoke-all-tenant-tx")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	secondaryTenantID, _ := testpkg.CreateTestTenant(t, db)
	testpkg.MapAccountToTenant(t, db, account.ID, secondaryTenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID, secondaryTenantID)
	})

	_, _, err = service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	other := &authModels.Token{
		AccountID:   account.ID,
		Token:       uniqueTestName("other-school-live"),
		Expiry:      time.Now().Add(time.Hour),
		PortalScope: authModels.PortalScopeTenant,
	}
	other.SetTenantID(secondaryTenantID)
	_, err = db.NewInsert().Model(other).ModelTableExpr("auth.tokens").Exec(context.Background())
	require.NoError(t, err)
	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/tx-school-a", "family-a")
	insertStaffPush(t, db, account.ID, secondaryTenantID, "https://fcm.googleapis.com/tx-school-b", "family-b")

	require.NoError(t, tenant.WithTenantTx(ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return service.RevokeAllTokens(txCtx, int(account.ID))
	}))

	count, err := db.NewSelect().TableExpr("auth.tokens").Where("account_id = ?", account.ID).Count(context.Background())
	require.NoError(t, err)
	require.Zero(t, count)
	require.Equal(t, 0, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/tx-school-a"))
	require.Equal(t, 0, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/tx-school-b"))
}

func TestSessionCapAppliesAcrossSchoolsOnSwitchTenant(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	ctx := testpkg.TenantContext(1)
	email, username := uniqueTestCredentials("switch-cap")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureTestTenant(t, db, 2)
	testpkg.MapAccountToTenant(t, db, account.ID, 2)
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })

	for range 5 {
		_, _, err = service.Login(ctx, email, testPassword)
		require.NoError(t, err)
	}
	_, _, err = service.SwitchTenant(ctx, account.ID, "t2")
	require.NoError(t, err)

	count, err := db.NewSelect().
		TableExpr("auth.tokens").
		Where("account_id = ?", account.ID).
		Where("rotated_at IS NULL").
		Where("expiry > NOW()").
		Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 5, count, "switch-tenant must share the staff portal cap across schools")
}

func TestCleanupExpiredTokensRemovesOrphanPush(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("orphan-push")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/orphan-family", "missing-family")
	_, err = service.CleanupExpiredTokens(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/orphan-family"))
}

func TestDeactivateAccountFromAdminTxRemovesPush(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("admin-deact-push")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	secondaryTenantID, _ := testpkg.CreateTestTenant(t, db)
	testpkg.MapAccountToTenant(t, db, account.ID, secondaryTenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID, secondaryTenantID)
	})

	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/admin-deact-a", "family-a")
	insertStaffPush(t, db, account.ID, secondaryTenantID, "https://fcm.googleapis.com/admin-deact-b", "family-b")

	require.NoError(t, tenant.WithAdminTx(ctx, db, func(adminCtx context.Context, _ bun.Tx) error {
		return service.DeactivateAccount(adminCtx, int(account.ID))
	}))

	require.Equal(t, 0, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/admin-deact-a"))
	require.Equal(t, 0, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/admin-deact-b"))
}

func countStaffPush(t *testing.T, db *bun.DB, accountID int64, endpoint string) int {
	t.Helper()
	return countPush(t, db, accountID, iotModels.PushPortalStaff, endpoint)
}

func countParentPush(t *testing.T, db *bun.DB, accountID int64, endpoint string) int {
	t.Helper()
	return countPush(t, db, accountID, iotModels.PushPortalParent, endpoint)
}

func countPush(t *testing.T, db *bun.DB, accountID int64, portal, endpoint string) int {
	t.Helper()
	count, err := db.NewSelect().
		Model((*iotModels.PushSubscription)(nil)).
		ModelTableExpr(`iot.push_subscriptions AS "push_subscription"`).
		Where("account_id = ?", accountID).
		Where("portal = ?", portal).
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

func TestSessionCapRemovesStaffPushForEvictedFamily(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("session-cap-push")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	_, _, err = service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	firstFamily := tokenFamilyID(t, db, account.ID, 0)
	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/evicted-device", firstFamily)

	_, _, err = service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	keptFamily := tokenFamilyID(t, db, account.ID, 1)
	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/kept-device", keptFamily)

	for range 4 {
		_, _, err = service.Login(ctx, email, testPassword)
		require.NoError(t, err)
	}

	require.Equal(t, 0, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/evicted-device"))
	require.Equal(t, 1, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/kept-device"))
}

func TestLogoutRemovesUnboundStaffPushAtSessionTenant(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("logout-unbound-push")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	secondaryTenantID, _ := testpkg.CreateTestTenant(t, db)
	testpkg.MapAccountToTenant(t, db, account.ID, secondaryTenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID, secondaryTenantID)
	})

	_, refreshToken, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/unbound-here", "")
	insertStaffPush(t, db, account.ID, secondaryTenantID, "https://fcm.googleapis.com/unbound-other", "")

	require.NoError(t, service.LogoutWithAudit(ctx, refreshToken, "", ""))

	require.Equal(t, 0, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/unbound-here"))
	require.Equal(t, 1, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/unbound-other"))
}

func TestRoleChangeKeepsStaffPushAtOtherSchools(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("role-change-push")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	secondaryTenantID, _ := testpkg.CreateTestTenant(t, db)
	testpkg.MapAccountToTenant(t, db, account.ID, secondaryTenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID, secondaryTenantID)
	})

	_, _, err = service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	localFamily := tokenFamilyID(t, db, account.ID, 0)
	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/this-school", localFamily)
	insertStaffPush(t, db, account.ID, secondaryTenantID, "https://fcm.googleapis.com/other-school", "other-family")

	role, err := service.CreateRole(ctx, uniqueTestName("role-change-push"), "limit push wipe", testpkg.StrPtr(authModels.BaseRoleUser))
	require.NoError(t, err)
	require.NoError(t, service.AssignRoleToAccount(ctx, int(account.ID), int(role.ID)))

	require.Equal(t, 0, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/this-school"))
	require.Equal(t, 1, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/other-school"))
}

func TestAssignRoleFromAdminTxKeepsOtherSchoolSessions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("admin-role-other-school")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	secondaryTenantID, _ := testpkg.CreateTestTenant(t, db)
	testpkg.MapAccountToTenant(t, db, account.ID, secondaryTenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID, secondaryTenantID)
	})

	_, _, err = service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	other := &authModels.Token{
		AccountID:   account.ID,
		Token:       uniqueTestName("role-other-school"),
		Expiry:      time.Now().Add(time.Hour),
		PortalScope: authModels.PortalScopeTenant,
	}
	other.SetTenantID(secondaryTenantID)
	_, err = db.NewInsert().Model(other).ModelTableExpr("auth.tokens").Exec(context.Background())
	require.NoError(t, err)

	role, err := service.CreateRole(ctx, uniqueTestName("admin-role-other"), "keep other school", testpkg.StrPtr(authModels.BaseRoleUser))
	require.NoError(t, err)
	require.NoError(t, tenant.WithAdminTx(ctx, db, func(adminCtx context.Context, _ bun.Tx) error {
		return service.AssignRoleToAccount(tenant.WithTenantID(adminCtx, tenantID), int(account.ID), int(role.ID))
	}))

	count, err := db.NewSelect().
		TableExpr("auth.tokens").
		Where("account_id = ?", account.ID).
		Where("tenant_id = ?", secondaryTenantID).
		Where("rotated_at IS NULL").
		Where("expiry > NOW()").
		Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func uniqueTestName(prefix string) string {
	email, _ := uniqueTestCredentials(prefix)
	return email
}

func TestLogoutRemovesParentPushForFamily(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("logout-parent-push")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	assignSeededRole(t, db, account.ID, tenantID, authModels.BaseRoleGuardian)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	_, firstRefresh, err := service.LoginParent(ctx, email, testPassword)
	require.NoError(t, err)
	_, secondRefresh, err := service.LoginParent(ctx, email, testPassword)
	require.NoError(t, err)

	firstFamily := tokenFamilyID(t, db, account.ID, 0)
	secondFamily := tokenFamilyID(t, db, account.ID, 1)
	require.NotEqual(t, firstFamily, secondFamily)

	insertParentPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/parent-this", firstFamily)
	insertParentPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/parent-other", secondFamily)
	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/staff-unrelated", "staff-family")

	require.NoError(t, service.LogoutWithAudit(ctx, firstRefresh, "", ""))

	require.Equal(t, 0, countParentPush(t, db, account.ID, "https://fcm.googleapis.com/parent-this"))
	require.Equal(t, 1, countParentPush(t, db, account.ID, "https://fcm.googleapis.com/parent-other"))
	require.Equal(t, 1, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/staff-unrelated"))

	_, _, err = service.RefreshToken(ctx, secondRefresh)
	require.NoError(t, err)
}

func TestLogoutRemovesUnboundParentPushAtSessionTenant(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("logout-unbound-parent")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	assignSeededRole(t, db, account.ID, tenantID, authModels.BaseRoleGuardian)
	secondaryTenantID, _ := testpkg.CreateTestTenant(t, db)
	testpkg.MapAccountToTenant(t, db, account.ID, secondaryTenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID, secondaryTenantID)
	})

	_, refreshToken, err := service.LoginParent(ctx, email, testPassword)
	require.NoError(t, err)
	insertParentPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/parent-unbound-here", "")
	insertParentPush(t, db, account.ID, secondaryTenantID, "https://fcm.googleapis.com/parent-unbound-other", "")
	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/staff-unbound-here", "")

	require.NoError(t, service.LogoutWithAudit(ctx, refreshToken, "", ""))

	require.Equal(t, 0, countParentPush(t, db, account.ID, "https://fcm.googleapis.com/parent-unbound-here"))
	require.Equal(t, 1, countParentPush(t, db, account.ID, "https://fcm.googleapis.com/parent-unbound-other"))
	require.Equal(t, 1, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/staff-unbound-here"))
}

func TestSessionCapRemovesUnboundStaffPushAtEvictedTenant(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("session-cap-unbound")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	_, _, err = service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/unbound-evicted", "")

	for range 5 {
		_, _, err = service.Login(ctx, email, testPassword)
		require.NoError(t, err)
	}

	require.Equal(t, 0, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/unbound-evicted"))
}

func TestSessionCapRemovesParentPushForEvictedFamily(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("session-cap-parent-push")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	assignSeededRole(t, db, account.ID, tenantID, authModels.BaseRoleGuardian)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	_, _, err = service.LoginParent(ctx, email, testPassword)
	require.NoError(t, err)
	firstFamily := tokenFamilyID(t, db, account.ID, 0)
	insertParentPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/parent-evicted", firstFamily)

	_, _, err = service.LoginParent(ctx, email, testPassword)
	require.NoError(t, err)
	keptFamily := tokenFamilyID(t, db, account.ID, 1)
	insertParentPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/parent-kept", keptFamily)

	for range 4 {
		_, _, err = service.LoginParent(ctx, email, testPassword)
		require.NoError(t, err)
	}

	require.Equal(t, 0, countParentPush(t, db, account.ID, "https://fcm.googleapis.com/parent-evicted"))
	require.Equal(t, 1, countParentPush(t, db, account.ID, "https://fcm.googleapis.com/parent-kept"))
}

func TestRevokeAllTokensClearsParentPushAcrossTenants(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("revoke-all-parent-push")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	secondaryTenantID := account.ID + 1_000_000_000
	testpkg.EnsureTestTenant(t, db, secondaryTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, secondaryTenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID, secondaryTenantID)
	})

	insertParentPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/parent-school-a", "parent-family-a")
	insertParentPush(t, db, account.ID, secondaryTenantID, "https://fcm.googleapis.com/parent-school-b", "parent-family-b")

	require.NoError(t, service.RevokeAllTokens(ctx, int(account.ID)))

	require.Equal(t, 0, countParentPush(t, db, account.ID, "https://fcm.googleapis.com/parent-school-a"))
	require.Equal(t, 0, countParentPush(t, db, account.ID, "https://fcm.googleapis.com/parent-school-b"))
}

func TestLogoutUnknownScopeRemovesBothUnboundPortals(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("logout-unknown-push")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	_, refreshToken, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	_, err = db.NewUpdate().
		TableExpr("auth.tokens").
		Set("portal_scope = ?", authModels.PortalScopeUnknown).
		Where("account_id = ?", account.ID).
		Exec(context.Background())
	require.NoError(t, err)

	insertParentPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/unknown-parent-unbound", "")
	insertStaffPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/unknown-staff-unbound", "")

	require.NoError(t, service.LogoutWithAudit(ctx, refreshToken, "", ""))

	require.Equal(t, 0, countParentPush(t, db, account.ID, "https://fcm.googleapis.com/unknown-parent-unbound"))
	require.Equal(t, 0, countStaffPush(t, db, account.ID, "https://fcm.googleapis.com/unknown-staff-unbound"))
}

func TestSessionCapLeavesUnknownSessionsIsolated(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("session-cap-unknown")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	for i := range 5 {
		legacy := &authModels.Token{
			AccountID:   account.ID,
			Token:       uniqueTestName("unknown-session") + "-" + string(rune('a'+i)),
			Expiry:      time.Now().Add(time.Hour),
			PortalScope: authModels.PortalScopeUnknown,
		}
		legacy.SetTenantID(tenantID)
		_, err = db.NewInsert().Model(legacy).ModelTableExpr("auth.tokens").Exec(context.Background())
		require.NoError(t, err)
	}

	_, _, err = service.Login(ctx, email, testPassword)
	require.NoError(t, err)

	assertTokenCountByPortal(t, db, account.ID, authModels.PortalScopeUnknown, 5)
	assertTokenCountByPortal(t, db, account.ID, authModels.PortalScopeTenant, 1)
}

func TestRevokeAllTokensDeletesSessionsAcrossTenants(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("revoke-all-tokens")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	secondaryTenantID, _ := testpkg.CreateTestTenant(t, db)
	testpkg.MapAccountToTenant(t, db, account.ID, secondaryTenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID, secondaryTenantID)
	})

	_, _, err = service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	other := &authModels.Token{
		AccountID:   account.ID,
		Token:       uniqueTestName("other-school-session"),
		Expiry:      time.Now().Add(time.Hour),
		PortalScope: authModels.PortalScopeTenant,
	}
	other.SetTenantID(secondaryTenantID)
	_, err = db.NewInsert().Model(other).ModelTableExpr("auth.tokens").Exec(context.Background())
	require.NoError(t, err)

	require.NoError(t, service.RevokeAllTokens(ctx, int(account.ID)))

	count, err := db.NewSelect().TableExpr("auth.tokens").Where("account_id = ?", account.ID).Count(context.Background())
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestOrphanCleanupKeepsUnboundParentPushAtOtherSchool(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("parent-unbound-other")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	assignSeededRole(t, db, account.ID, tenantID, authModels.BaseRoleGuardian)
	secondaryTenantID, _ := testpkg.CreateTestTenant(t, db)
	testpkg.MapAccountToTenant(t, db, account.ID, secondaryTenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID, secondaryTenantID)
	})

	_, _, err = service.LoginParent(ctx, email, testPassword)
	require.NoError(t, err)
	insertParentPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/parent-unbound-home", "")
	insertParentPush(t, db, account.ID, secondaryTenantID, "https://fcm.googleapis.com/parent-unbound-other", "")

	_, err = service.CleanupExpiredTokens(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, countParentPush(t, db, account.ID, "https://fcm.googleapis.com/parent-unbound-home"))
	require.Equal(t, 1, countParentPush(t, db, account.ID, "https://fcm.googleapis.com/parent-unbound-other"))
}

func TestRevokeAllFromAdminTxWithTenantDeletesOtherSchoolTokens(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("admin-tx-tenant-wipe")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	secondaryTenantID, _ := testpkg.CreateTestTenant(t, db)
	testpkg.MapAccountToTenant(t, db, account.ID, secondaryTenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID, secondaryTenantID)
	})

	_, _, err = service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	other := &authModels.Token{
		AccountID:   account.ID,
		Token:       uniqueTestName("admin-tx-other-school"),
		Expiry:      time.Now().Add(time.Hour),
		PortalScope: authModels.PortalScopeTenant,
	}
	other.SetTenantID(secondaryTenantID)
	_, err = db.NewInsert().Model(other).ModelTableExpr("auth.tokens").Exec(context.Background())
	require.NoError(t, err)

	require.NoError(t, tenant.WithAdminTx(ctx, db, func(adminCtx context.Context, _ bun.Tx) error {
		return service.RevokeAllTokens(adminCtx, int(account.ID))
	}))

	count, err := db.NewSelect().TableExpr("auth.tokens").Where("account_id = ?", account.ID).Count(context.Background())
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestCleanupExpiredTokensDoesNotWipeReactivatedSessions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("reactivate-wipe")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	require.NoError(t, tenant.WithTenantTx(ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return service.DeactivateAccount(txCtx, int(account.ID))
	}))
	require.NoError(t, service.ActivateAccount(ctx, int(account.ID)))
	_, _, err = service.Login(ctx, email, testPassword)
	require.NoError(t, err)

	_, err = service.CleanupExpiredTokens(ctx)
	require.NoError(t, err)

	count, err := db.NewSelect().
		TableExpr("auth.tokens").
		Where("account_id = ?", account.ID).
		Where("rotated_at IS NULL").
		Where("expiry > NOW()").
		Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestActivateAccountClearsPendingAccountWideWipe(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("activate-clear-pending")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	insertPendingAccountWideWipe(t, db, account.ID, tenantID, "account_deactivated", time.Now())
	require.NoError(t, service.ActivateAccount(ctx, int(account.ID)))

	pending, err := db.NewSelect().
		TableExpr("audit.auth_events").
		Where("account_id = ?", account.ID).
		Where("event_type = ?", "token_revoked").
		Where(`metadata @> ?`, `{"pending_account_wide_wipe":true}`).
		Count(context.Background())
	require.NoError(t, err)
	require.Zero(t, pending)
}

func TestCleanupExpiredTokensLeavesSessionsCreatedAfterPendingWipe(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("stale-pending-login")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	insertPendingAccountWideWipe(t, db, account.ID, tenantID, "administrative_revoke", time.Now().Add(-time.Minute))
	_, _, err = service.Login(ctx, email, testPassword)
	require.NoError(t, err)

	_, err = service.CleanupExpiredTokens(ctx)
	require.NoError(t, err)

	count, err := db.NewSelect().
		TableExpr("auth.tokens").
		Where("account_id = ?", account.ID).
		Where("rotated_at IS NULL").
		Where("expiry > NOW()").
		Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestCleanupExpiredTokensRevokesRefreshedFamilyAfterPendingWipe(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("pending-refresh-successor")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	familyID := uniqueTestName("pre-revoke-family")
	cutoff := time.Now().Add(-time.Minute)
	_, err = db.NewRaw(`
		INSERT INTO auth.tokens (account_id, token, expiry, tenant_id, portal_scope, family_id, generation, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?)
	`, account.ID, uniqueTestName("pre-revoke-succ"), time.Now().Add(14*24*time.Hour),
		tenantID, authModels.PortalScopeTenant, familyID, time.Now()).
		Exec(context.Background())
	require.NoError(t, err)
	insertPendingAccountWideWipe(t, db, account.ID, tenantID, "administrative_revoke", cutoff)

	_, err = service.CleanupExpiredTokens(ctx)
	require.NoError(t, err)

	count, err := db.NewSelect().
		TableExpr("auth.tokens").
		Where("account_id = ?", account.ID).
		Where("rotated_at IS NULL").
		Where("expiry > NOW()").
		Count(context.Background())
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestCleanupExpiredTokensRetriesPendingWipeOlderThanSevenDays(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("old-pending-wipe")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	tokenCreatedAt := time.Now().Add(-9 * 24 * time.Hour)
	wipeAt := time.Now().Add(-8 * 24 * time.Hour)
	_, err = db.NewRaw(`
		INSERT INTO auth.tokens (account_id, token, expiry, tenant_id, portal_scope, family_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, account.ID, uniqueTestName("old-pre-revoke"), time.Now().Add(14*24*time.Hour),
		tenantID, authModels.PortalScopeTenant, uniqueTestName("old-family"), tokenCreatedAt).
		Exec(context.Background())
	require.NoError(t, err)
	insertPendingAccountWideWipe(t, db, account.ID, tenantID, "password_reset", wipeAt)

	_, err = service.CleanupExpiredTokens(ctx)
	require.NoError(t, err)

	count, err := db.NewSelect().
		TableExpr("auth.tokens").
		Where("account_id = ?", account.ID).
		Where("rotated_at IS NULL").
		Where("expiry > NOW()").
		Count(context.Background())
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestCleanupExpiredTokensKeepsParentPushForUnknownSession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("unknown-parent-push")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		testpkg.CleanupAuthFixtures(t, db, account.ID)
		testpkg.CleanupTestTenant(t, db, tenantID)
	})

	legacy := &authModels.Token{
		AccountID:   account.ID,
		Token:       uniqueTestName("legacy-parent-session"),
		Expiry:      time.Now().Add(time.Hour),
		PortalScope: authModels.PortalScopeUnknown,
	}
	legacy.SetTenantID(tenantID)
	_, err = db.NewInsert().Model(legacy).ModelTableExpr("auth.tokens").Exec(context.Background())
	require.NoError(t, err)
	insertParentPush(t, db, account.ID, tenantID, "https://fcm.googleapis.com/legacy-parent", "")

	_, err = service.CleanupExpiredTokens(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, countParentPush(t, db, account.ID, "https://fcm.googleapis.com/legacy-parent"))
}

func insertPendingAccountWideWipe(t *testing.T, db *bun.DB, accountID, tenantID int64, reason string, createdAt time.Time) {
	t.Helper()
	_, err := db.NewRaw(`
		INSERT INTO audit.auth_events (tenant_id, account_id, event_type, success, ip_address, metadata, created_at)
		VALUES (?, ?, 'token_revoked', true, '0.0.0.0', jsonb_build_object('reason', ?::text, 'pending_account_wide_wipe', true), ?)
	`, tenantID, accountID, reason, createdAt).Exec(context.Background())
	require.NoError(t, err)
}
