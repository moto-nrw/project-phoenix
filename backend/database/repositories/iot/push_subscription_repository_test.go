package iot_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	iotRepo "github.com/moto-nrw/project-phoenix/database/repositories/iot"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func cleanupPushSubscriptions(t *testing.T, db *bun.DB, accountIDs ...int64) {
	t.Helper()
	ctx := context.Background()
	_, err := db.NewDelete().
		Model((*iotModels.PushSubscription)(nil)).
		ModelTableExpr("iot.push_subscriptions").
		Where("account_id IN (?)", bun.List(accountIDs)).
		Exec(ctx)
	require.NoError(t, err)
}

func createAccountTenantMapping(t *testing.T, db *bun.DB, accountID, tenantID int64) {
	t.Helper()
	now := time.Now()
	mapping := &authModels.AccountTenant{
		AccountID:   accountID,
		TenantID:    tenantID,
		Status:      authModels.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	_, err := db.NewInsert().
		Model(mapping).
		ModelTableExpr("auth.account_tenants").
		Exec(context.Background())
	require.NoError(t, err)
}

func setAccountTenantStatus(t *testing.T, db *bun.DB, status string, accountIDs ...int64) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("auth.account_tenants").
		Set("status = ?", status).
		Where("tenant_id = ?", 1).
		Where("account_id IN (?)", bun.List(accountIDs)).
		Exec(context.Background())
	require.NoError(t, err)
}

func newSubscription(accountID int64, portal, endpoint string) *iotModels.PushSubscription {
	sub := &iotModels.PushSubscription{
		AccountID: accountID,
		Portal:    portal,
		Endpoint:  endpoint,
		P256dh:    "p256dh-key",
		Auth:      "auth-key",
		UserAgent: "test-agent",
	}
	sub.SetTenantID(1)
	return sub
}

func subscriptionsForAccount(subs []*iotModels.PushSubscription, accountID int64) []*iotModels.PushSubscription {
	var matches []*iotModels.PushSubscription
	for _, sub := range subs {
		if sub.AccountID == accountID {
			matches = append(matches, sub)
		}
	}
	return matches
}

func hasSubscriptionEndpoint(subs []*iotModels.PushSubscription, endpoint string) bool {
	for _, sub := range subs {
		if sub.Endpoint == endpoint {
			return true
		}
	}
	return false
}

func TestPushSubscriptionRepository(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := iotRepo.NewPushSubscriptionRepository(db)
	ctx := tenant.WithTenantID(context.Background(), 1)
	otherTenantCtx := tenant.WithTenantID(context.Background(), 2)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("push-%d@example.com", time.Now().UnixNano()))
	guardian := testpkg.CreateTestAccount(t, db, fmt.Sprintf("push-parent-%d@example.com", time.Now().UnixNano()))
	defer testpkg.CleanupAuthFixtures(t, db, account.ID, guardian.ID)
	defer cleanupPushSubscriptions(t, db, account.ID, guardian.ID)
	createAccountTenantMapping(t, db, account.ID, 1)
	createAccountTenantMapping(t, db, guardian.ID, 1)

	endpoint := fmt.Sprintf("https://fcm.googleapis.com/fcm/send/%d", time.Now().UnixNano())

	t.Run("upsert inserts and refreshes by (tenant, endpoint)", func(t *testing.T) {
		require.NoError(t, repo.Upsert(ctx, newSubscription(account.ID, iotModels.PushPortalStaff, endpoint)))

		rotated := newSubscription(account.ID, iotModels.PushPortalStaff, endpoint)
		rotated.P256dh = "rotated-p256dh"
		require.NoError(t, repo.Upsert(ctx, rotated))

		subs, err := repo.FindForTenantStaff(ctx)
		require.NoError(t, err)
		mine := subscriptionsForAccount(subs, account.ID)
		require.Len(t, mine, 1, "upsert must not duplicate the endpoint")
		assert.Equal(t, "rotated-p256dh", mine[0].P256dh)
	})

	t.Run("tenant filter hides rows from other tenants", func(t *testing.T) {
		subs, err := repo.FindForTenantStaff(otherTenantCtx)
		require.NoError(t, err)
		assert.Empty(t, subscriptionsForAccount(subs, account.ID))
	})

	t.Run("guardian finder returns only parent-portal rows of that account", func(t *testing.T) {
		parentEndpoint := endpoint + "/parent"
		require.NoError(t, repo.Upsert(ctx, newSubscription(guardian.ID, iotModels.PushPortalParent, parentEndpoint)))

		subs, err := repo.FindForGuardian(ctx, guardian.ID)
		require.NoError(t, err)
		require.Len(t, subs, 1)
		assert.Equal(t, parentEndpoint, subs[0].Endpoint)

		// The staff finder must not return parent-portal rows.
		staffSubs, err := repo.FindForTenantStaff(ctx)
		require.NoError(t, err)
		assert.Empty(t, subscriptionsForAccount(staffSubs, guardian.ID))
	})

	t.Run("admin finder returns only admin-role accounts", func(t *testing.T) {
		subs, err := repo.FindForTenantAdmins(ctx)
		require.NoError(t, err)
		assert.Empty(t, subscriptionsForAccount(subs, account.ID), "account without admin role must not appear")

		// Grant the seeded admin role and expect the subscription to appear.
		var adminRoleID int64
		err = db.NewSelect().
			ColumnExpr("id").
			TableExpr("auth.roles").
			Where("name = ?", authModels.BaseRoleAdmin).
			Scan(context.Background(), &adminRoleID)
		require.NoError(t, err)

		roleAssignment := &authModels.AccountRole{AccountID: account.ID, RoleID: adminRoleID}
		roleAssignment.SetTenantID(1)
		_, err = db.NewInsert().Model(roleAssignment).ModelTableExpr(`auth.account_roles`).Exec(context.Background())
		require.NoError(t, err)

		subs, err = repo.FindForTenantAdmins(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, subscriptionsForAccount(subs, account.ID), "admin-role account's staff subscription must appear")
	})

	t.Run("recipient finders exclude inactive tenant mappings", func(t *testing.T) {
		setAccountTenantStatus(t, db, authModels.AccountTenantStatusInactive, account.ID, guardian.ID)
		defer setAccountTenantStatus(t, db, authModels.AccountTenantStatusActive, account.ID, guardian.ID)

		staffSubs, err := repo.FindForTenantStaff(ctx)
		require.NoError(t, err)
		assert.Empty(t, subscriptionsForAccount(staffSubs, account.ID))

		adminSubs, err := repo.FindForTenantAdmins(ctx)
		require.NoError(t, err)
		assert.Empty(t, subscriptionsForAccount(adminSubs, account.ID))

		guardianSubs, err := repo.FindForGuardian(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Empty(t, guardianSubs)
	})

	t.Run("delete by endpoint is scoped to the account", func(t *testing.T) {
		// Deleting with the wrong account must not remove the row.
		require.NoError(t, repo.DeleteByEndpoint(ctx, guardian.ID, endpoint))
		subs, err := repo.FindForTenantStaff(ctx)
		require.NoError(t, err)
		require.True(t, hasSubscriptionEndpoint(subs, endpoint))

		require.NoError(t, repo.DeleteByEndpoint(ctx, account.ID, endpoint))
		subs, err = repo.FindForTenantStaff(ctx)
		require.NoError(t, err)
		assert.False(t, hasSubscriptionEndpoint(subs, endpoint))
	})
}

func TestPushSubscriptionRepositoryEffectiveAdmins(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := iotRepo.NewPushSubscriptionRepository(db)
	ctx := tenant.WithTenantID(context.Background(), 1)
	directAdmin := testpkg.CreateTestAccount(t, db, fmt.Sprintf("push-direct-admin-%d@example.com", time.Now().UnixNano()))
	roleAdmin := testpkg.CreateTestAccount(t, db, fmt.Sprintf("push-role-admin-%d@example.com", time.Now().UnixNano()))
	ordinary := testpkg.CreateTestAccount(t, db, fmt.Sprintf("push-ordinary-%d@example.com", time.Now().UnixNano()))
	defer testpkg.CleanupAuthFixtures(t, db, directAdmin.ID, roleAdmin.ID, ordinary.ID)
	defer cleanupPushSubscriptions(t, db, directAdmin.ID, roleAdmin.ID, ordinary.ID)

	for accountID, suffix := range map[int64]string{
		directAdmin.ID: "direct-admin",
		roleAdmin.ID:   "role-admin",
		ordinary.ID:    "ordinary",
	} {
		createAccountTenantMapping(t, db, accountID, 1)
		require.NoError(t, repo.Upsert(ctx, newSubscription(
			accountID,
			iotModels.PushPortalStaff,
			"https://fcm.googleapis.com/fcm/send/"+suffix,
		)))
	}

	var adminWildcardID, fullAccessID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.permissions").
		Where("resource = 'admin' AND action = '*'").
		Scan(context.Background(), &adminWildcardID))
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.permissions").
		Where("resource = '*' AND action = '*'").
		Scan(context.Background(), &fullAccessID))

	directGrant := &authModels.AccountPermission{
		AccountID:    directAdmin.ID,
		PermissionID: adminWildcardID,
		Granted:      true,
	}
	directGrant.SetTenantID(1)
	_, err := db.NewInsert().Model(directGrant).ModelTableExpr("auth.account_permissions").Exec(context.Background())
	require.NoError(t, err)

	wildcardRole := testpkg.CreateTestRole(t, db, "Push Full Access")
	defer testpkg.CleanupRoleRecords(t, db, wildcardRole.ID)
	_, err = db.NewInsert().
		Model(&authModels.RolePermission{RoleID: wildcardRole.ID, PermissionID: fullAccessID}).
		ModelTableExpr("auth.role_permissions").
		Exec(context.Background())
	require.NoError(t, err)
	roleGrant := &authModels.AccountRole{AccountID: roleAdmin.ID, RoleID: wildcardRole.ID}
	roleGrant.SetTenantID(1)
	_, err = db.NewInsert().Model(roleGrant).ModelTableExpr("auth.account_roles").Exec(context.Background())
	require.NoError(t, err)

	subs, err := repo.FindForTenantAdmins(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, subscriptionsForAccount(subs, directAdmin.ID), "direct admin:* permission must grant admin push")
	assert.NotEmpty(t, subscriptionsForAccount(subs, roleAdmin.ID), "role-based *:* permission must grant admin push")
	assert.Empty(t, subscriptionsForAccount(subs, ordinary.ID), "ordinary staff must not receive admin push")

	var tenantRoleSubs []*iotModels.PushSubscription
	err = tenant.WithTenantTx(context.Background(), db, 1, func(txCtx context.Context, _ bun.Tx) error {
		tenantRoleSubs, err = repo.FindForTenantAdmins(txCtx)
		return err
	})
	require.NoError(t, err)
	assert.NotEmpty(t, subscriptionsForAccount(tenantRoleSubs, directAdmin.ID))
	assert.NotEmpty(t, subscriptionsForAccount(tenantRoleSubs, roleAdmin.ID))
}
