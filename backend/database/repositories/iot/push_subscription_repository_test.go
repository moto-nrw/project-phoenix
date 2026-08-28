package iot_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	iotRepo "github.com/moto-nrw/project-phoenix/database/repositories/iot"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
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
	// Upsert: since #2419 CreateTestAccount already maps a fixture account to
	// the tenant of the test that created it.
	_, err := db.NewInsert().
		Model(mapping).
		ModelTableExpr("auth.account_tenants").
		On("CONFLICT (account_id, tenant_id) DO UPDATE").
		Set("status = EXCLUDED.status, activated_at = EXCLUDED.activated_at").
		Exec(context.Background())
	require.NoError(t, err)
}

func createPushTestSchool(t *testing.T, db *bun.DB) *platformModels.School {
	t.Helper()
	var organizationID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("platform.organizations").
		OrderExpr("id ASC").
		Limit(1).
		Scan(context.Background(), &organizationID))

	suffix := time.Now().UnixNano()
	school := &platformModels.School{
		OrganizationID: organizationID,
		Name:           fmt.Sprintf("Push Test School %d", suffix),
		Slug:           fmt.Sprintf("push-test-%d", suffix),
		Subdomain:      fmt.Sprintf("push-test-%d", suffix),
		Active:         true,
	}
	_, err := db.NewInsert().
		Model(school).
		ModelTableExpr("platform.schools").
		Exec(context.Background())
	require.NoError(t, err)
	require.Positive(t, school.ID)

	t.Cleanup(func() {
		_, err := db.NewDelete().
			Model((*iotModels.PushSubscription)(nil)).
			ModelTableExpr("iot.push_subscriptions").
			Where("tenant_id = ?", school.ID).
			Exec(context.Background())
		require.NoError(t, err)
		_, err = db.NewDelete().
			TableExpr("platform.schools").
			Where("id = ?", school.ID).
			Exec(context.Background())
		require.NoError(t, err)
	})
	return school
}

func setAccountTenantStatus(t *testing.T, db *bun.DB, status string, accountIDs ...int64) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("auth.account_tenants").
		Set("status = ?", status).
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("account_id IN (?)", bun.List(accountIDs)).
		Exec(context.Background())
	require.NoError(t, err)
}

func setAccountActive(t *testing.T, db *bun.DB, active bool, accountIDs ...int64) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("auth.accounts").
		Set("active = ?", active).
		Where("id IN (?)", bun.List(accountIDs)).
		Exec(context.Background())
	require.NoError(t, err)
}

func assignSystemRole(t *testing.T, db *bun.DB, accountID, tenantID int64, roleName string) {
	t.Helper()
	var roleID int64
	err := db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("name = ?", roleName).
		Scan(context.Background(), &roleID)
	require.NoError(t, err)

	roleAssignment := &authModels.AccountRole{AccountID: accountID, RoleID: roleID}
	roleAssignment.SetTenantID(tenantID)
	_, err = db.NewInsert().
		Model(roleAssignment).
		ModelTableExpr("auth.account_roles").
		Exec(context.Background())
	require.NoError(t, err)
}

func newSubscription(tb testing.TB, accountID int64, portal, endpoint string) *iotModels.PushSubscription {
	sub := &iotModels.PushSubscription{
		AccountID: accountID,
		Portal:    portal,
		Endpoint:  endpoint,
		P256dh:    "p256dh-key",
		Auth:      "auth-key",
		UserAgent: "test-agent",
	}
	sub.SetTenantID(testpkg.Tenant(tb))
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := iotRepo.NewPushSubscriptionRepository(db)
	ctx := testpkg.Ctx(t)
	otherTenantCtx := tenant.WithTenantID(context.Background(), 2)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("push-%d@example.com", time.Now().UnixNano()))
	guardian := testpkg.CreateTestAccount(t, db, fmt.Sprintf("push-parent-%d@example.com", time.Now().UnixNano()))
	defer testpkg.CleanupAuthFixtures(t, db, account.ID, guardian.ID)
	defer cleanupPushSubscriptions(t, db, account.ID, guardian.ID)
	createAccountTenantMapping(t, db, account.ID, testpkg.Tenant(t))
	createAccountTenantMapping(t, db, guardian.ID, testpkg.Tenant(t))
	assignSystemRole(t, db, account.ID, testpkg.Tenant(t), authModels.BaseRoleUser)
	assignSystemRole(t, db, guardian.ID, testpkg.Tenant(t), authModels.BaseRoleGuardian)
	// School-portal delivery additionally requires the lehrkraft system role.
	testpkg.AssignLehrkraftSystemRole(t, db, account.ID, testpkg.Tenant(t))

	endpoint := fmt.Sprintf("https://fcm.googleapis.com/fcm/send/%d", time.Now().UnixNano())

	t.Run("upsert inserts and refreshes by (tenant, endpoint)", func(t *testing.T) {
		require.NoError(t, repo.Upsert(ctx, newSubscription(t, account.ID, iotModels.PushPortalStaff, endpoint)))

		rotated := newSubscription(t, account.ID, iotModels.PushPortalStaff, endpoint)
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
		require.NoError(t, repo.Upsert(ctx, newSubscription(t, guardian.ID, iotModels.PushPortalParent, parentEndpoint)))

		subs, err := repo.FindForGuardians(ctx, []int64{guardian.ID}, nil)
		require.NoError(t, err)
		require.Len(t, subs, 1)
		assert.Equal(t, parentEndpoint, subs[0].Endpoint)

		// A stale staff-portal row must not restore tenant delivery after the
		// account becomes guardian-only.
		require.NoError(t, repo.Upsert(ctx, newSubscription(t,
			guardian.ID,
			iotModels.PushPortalStaff,
			endpoint+"/guardian-staff",
		)))
		staffSubs, err := repo.FindForTenantStaff(ctx)
		require.NoError(t, err)
		assert.Empty(t, subscriptionsForAccount(staffSubs, guardian.ID))

		// Conversely, a parent-portal row for a staff-only account is not a
		// guardian recipient.
		require.NoError(t, repo.Upsert(ctx, newSubscription(t,
			account.ID,
			iotModels.PushPortalParent,
			endpoint+"/staff-parent",
		)))
		subs, err = repo.FindForGuardians(ctx, []int64{account.ID}, nil)
		require.NoError(t, err)
		assert.Empty(t, subs)
	})

	t.Run("child scope answers parent_portal.access for that child", func(t *testing.T) {
		chain := testpkg.CreateTestParentGuardianChain(t, db)
		defer cleanupPushSubscriptions(t, db, chain.AccountID)

		chainCtx := tenant.WithTenantID(context.Background(), chain.TenantID)
		childEndpoint := fmt.Sprintf("https://fcm.googleapis.com/fcm/send/child-%d", time.Now().UnixNano())
		require.NoError(t, repo.Upsert(chainCtx, newSubscription(t, chain.AccountID, iotModels.PushPortalParent, childEndpoint)))

		subs, err := repo.FindForGuardians(chainCtx, []int64{chain.AccountID}, []int64{chain.StudentID})
		require.NoError(t, err)
		require.Len(t, subs, 1, "a guardian with parent_portal.access is a recipient for their child")
		assert.Equal(t, childEndpoint, subs[0].Endpoint)

		// Another family's child in the same school: the relationship carries the
		// permission, so being a guardian somewhere is not access everywhere.
		otherFamily := testpkg.CreateTestParentGuardianChain(t, db)
		subs, err = repo.FindForGuardians(chainCtx, []int64{chain.AccountID}, []int64{otherFamily.StudentID})
		require.NoError(t, err)
		assert.Empty(t, subs, "no relationship to that child means no push about it")

		// Access revoked after the producer picked its audience — the account,
		// its tenant mapping, its guardian role and its device all stay intact.
		_, err = db.ExecContext(context.Background(),
			`UPDATE users.students_guardians SET permissions = permissions - 'parent_portal.access'
			 WHERE student_id = ? AND tenant_id = ?`, chain.StudentID, chain.TenantID)
		require.NoError(t, err)

		subs, err = repo.FindForGuardians(chainCtx, []int64{chain.AccountID}, []int64{chain.StudentID})
		require.NoError(t, err)
		assert.Empty(t, subs, "the sending transaction must not deliver on a revoked access")

		subs, err = repo.FindForGuardians(chainCtx, []int64{chain.AccountID}, nil)
		require.NoError(t, err)
		assert.Len(t, subs, 1, "an unscoped audience keeps the account-level rules it always had")

		// A multi-child scope — an appointment addressed to siblings — admits the
		// account while any one of them still permits it, and drops it once none
		// does. Restoring access to the family's own child shows both directions.
		_, err = db.ExecContext(context.Background(),
			`UPDATE users.students_guardians
			 SET permissions = permissions || '{"parent_portal.access": true}'::jsonb
			 WHERE student_id = ? AND tenant_id = ?`, chain.StudentID, chain.TenantID)
		require.NoError(t, err)

		subs, err = repo.FindForGuardians(chainCtx, []int64{chain.AccountID}, []int64{otherFamily.StudentID, chain.StudentID})
		require.NoError(t, err)
		assert.Len(t, subs, 1, "one permitted child on the appointment is enough to stay a recipient")
	})

	t.Run("parent endpoint cleanup spans tenants", func(t *testing.T) {
		otherSchool := createPushTestSchool(t, db)
		otherSchoolCtx := tenant.WithTenantID(context.Background(), otherSchool.ID)
		sharedEndpoint := endpoint + "/shared-parent-device"
		tenantOneSub := newSubscription(t, guardian.ID, iotModels.PushPortalParent, sharedEndpoint)
		tenantTwoSub := newSubscription(t, guardian.ID, iotModels.PushPortalParent, sharedEndpoint)
		tenantTwoSub.SetTenantID(otherSchool.ID)
		require.NoError(t, repo.Upsert(ctx, tenantOneSub))
		require.NoError(t, repo.Upsert(otherSchoolCtx, tenantTwoSub))

		require.NoError(t, tenant.WithAdminTx(context.Background(), db, func(adminCtx context.Context, _ bun.Tx) error {
			return repo.DeleteParentByEndpoint(adminCtx, sharedEndpoint)
		}))

		var remaining []*iotModels.PushSubscription
		require.NoError(t, db.NewSelect().
			Model(&remaining).
			ModelTableExpr(`iot.push_subscriptions AS "push_subscription"`).
			Where(`"push_subscription".endpoint = ?`, sharedEndpoint).
			Scan(context.Background()))
		assert.Empty(t, remaining)
	})

	t.Run("school endpoint cleanup spans tenants and leaves other portals alone", func(t *testing.T) {
		otherSchool := createPushTestSchool(t, db)
		otherSchoolCtx := tenant.WithTenantID(context.Background(), otherSchool.ID)
		sharedEndpoint := endpoint + "/shared-school-device"

		schoolHere := newSubscription(t, account.ID, iotModels.PushPortalSchool, sharedEndpoint)
		schoolThere := newSubscription(t, account.ID, iotModels.PushPortalSchool, sharedEndpoint)
		schoolThere.SetTenantID(otherSchool.ID)
		staffHere := newSubscription(t, account.ID, iotModels.PushPortalStaff, sharedEndpoint)
		require.NoError(t, repo.Upsert(ctx, schoolHere))
		require.NoError(t, repo.Upsert(otherSchoolCtx, schoolThere))
		require.NoError(t, repo.Upsert(ctx, staffHere))

		require.NoError(t, tenant.WithAdminTx(context.Background(), db, func(adminCtx context.Context, _ bun.Tx) error {
			return repo.DeleteSchoolByEndpointAcrossTenants(adminCtx, sharedEndpoint)
		}))

		var remaining []*iotModels.PushSubscription
		require.NoError(t, db.NewSelect().
			Model(&remaining).
			ModelTableExpr(`iot.push_subscriptions AS "push_subscription"`).
			Where(`"push_subscription".endpoint = ?`, sharedEndpoint).
			Scan(context.Background()))
		require.Len(t, remaining, 1, "the OGS registration of the same browser survives")
		assert.Equal(t, iotModels.PushPortalStaff, remaining[0].Portal)
	})

	t.Run("admin finder returns only admin-role accounts", func(t *testing.T) {
		subs, err := repo.FindForTenantAdmins(ctx)
		require.NoError(t, err)
		assert.Empty(t, subscriptionsForAccount(subs, account.ID), "account without admin role must not appear")

		// Grant the seeded admin role and expect the subscription to appear.
		assignSystemRole(t, db, account.ID, testpkg.Tenant(t), authModels.BaseRoleAdmin)

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

		guardianSubs, err := repo.FindForGuardians(ctx, []int64{guardian.ID}, nil)
		require.NoError(t, err)
		assert.Empty(t, guardianSubs)
	})

	t.Run("recipient finders exclude inactive accounts", func(t *testing.T) {
		setAccountActive(t, db, false, account.ID, guardian.ID)
		defer setAccountActive(t, db, true, account.ID, guardian.ID)

		staffSubs, err := repo.FindForTenantStaff(ctx)
		require.NoError(t, err)
		assert.Empty(t, subscriptionsForAccount(staffSubs, account.ID))

		adminSubs, err := repo.FindForTenantAdmins(ctx)
		require.NoError(t, err)
		assert.Empty(t, subscriptionsForAccount(adminSubs, account.ID))

		guardianSubs, err := repo.FindForGuardians(ctx, []int64{guardian.ID}, nil)
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

	t.Run("school endpoint deletion preserves a staff subscription", func(t *testing.T) {
		staffEndpoint := endpoint + "/staff-kept"
		require.NoError(t, repo.Upsert(ctx, newSubscription(t, account.ID, iotModels.PushPortalStaff, staffEndpoint)))

		require.NoError(t, repo.DeleteSchoolByEndpoint(ctx, account.ID, staffEndpoint))
		subs, err := repo.FindForTenantStaff(ctx)
		require.NoError(t, err)
		assert.True(t, hasSubscriptionEndpoint(subs, staffEndpoint))
	})

	t.Run("same endpoint can be registered in both staff portals", func(t *testing.T) {
		sharedEndpoint := endpoint + "/shared-portals"
		require.NoError(t, repo.Upsert(ctx, newSubscription(t, account.ID, iotModels.PushPortalStaff, sharedEndpoint)))
		require.NoError(t, repo.Upsert(ctx, newSubscription(t, account.ID, iotModels.PushPortalSchool, sharedEndpoint)))
		require.NoError(t, repo.Upsert(ctx, newSubscription(t, account.ID, iotModels.PushPortalParent, sharedEndpoint)))

		staffSubs, err := repo.FindForStaffAccounts(ctx, []int64{account.ID})
		require.NoError(t, err)
		assert.True(t, hasSubscriptionEndpoint(staffSubs, sharedEndpoint))

		schoolSubs, err := repo.FindForSchoolAccounts(ctx, []int64{account.ID})
		require.NoError(t, err)
		assert.True(t, hasSubscriptionEndpoint(schoolSubs, sharedEndpoint))

		require.NoError(t, repo.DeleteByEndpoint(ctx, account.ID, sharedEndpoint))
		schoolSubs, err = repo.FindForSchoolAccounts(ctx, []int64{account.ID})
		require.NoError(t, err)
		assert.True(t, hasSubscriptionEndpoint(schoolSubs, sharedEndpoint))

		require.NoError(t, repo.DeleteParentByAccountEndpoint(ctx, account.ID, sharedEndpoint))
		schoolSubs, err = repo.FindForSchoolAccounts(ctx, []int64{account.ID})
		require.NoError(t, err)
		assert.True(t, hasSubscriptionEndpoint(schoolSubs, sharedEndpoint))

		require.NoError(t, repo.DeleteSchoolByEndpoint(ctx, account.ID, sharedEndpoint))
	})

	t.Run("expired cleanup preserves a refreshed subscription", func(t *testing.T) {
		raceEndpoint := endpoint + "/refresh-race"
		require.NoError(t, repo.Upsert(ctx, newSubscription(t, account.ID, iotModels.PushPortalStaff, raceEndpoint)))

		var sentSnapshot iotModels.PushSubscription
		require.NoError(t, db.NewSelect().
			Model(&sentSnapshot).
			ModelTableExpr(`iot.push_subscriptions AS "push_subscription"`).
			Where(`"push_subscription".tenant_id = ?`, tenant.FromContext(ctx)).
			Where(`"push_subscription".endpoint = ?`, raceEndpoint).
			Scan(context.Background()))

		refreshed := newSubscription(t, account.ID, iotModels.PushPortalStaff, raceEndpoint)
		refreshed.P256dh = "refreshed-p256dh"
		refreshed.Auth = "refreshed-auth"
		require.NoError(t, repo.Upsert(ctx, refreshed))

		deleted, err := repo.DeleteExpiredIfUnchanged(ctx, &sentSnapshot)
		require.NoError(t, err)
		assert.False(t, deleted)

		var current iotModels.PushSubscription
		require.NoError(t, db.NewSelect().
			Model(&current).
			ModelTableExpr(`iot.push_subscriptions AS "push_subscription"`).
			Where(`"push_subscription".tenant_id = ?`, tenant.FromContext(ctx)).
			Where(`"push_subscription".endpoint = ?`, raceEndpoint).
			Scan(context.Background()))
		assert.Equal(t, "refreshed-p256dh", current.P256dh)
		assert.Equal(t, "refreshed-auth", current.Auth)

		rebound := newSubscription(t, guardian.ID, iotModels.PushPortalParent, raceEndpoint)
		rebound.P256dh = current.P256dh
		rebound.Auth = current.Auth
		require.NoError(t, repo.Upsert(ctx, rebound))

		deleted, err = repo.DeleteExpiredIfUnchanged(ctx, &current)
		require.NoError(t, err)
		assert.True(t, deleted)

		var reboundCurrent iotModels.PushSubscription
		require.NoError(t, db.NewSelect().
			Model(&reboundCurrent).
			ModelTableExpr(`iot.push_subscriptions AS "push_subscription"`).
			Where(`"push_subscription".tenant_id = ?`, tenant.FromContext(ctx)).
			Where(`"push_subscription".endpoint = ?`, raceEndpoint).
			Where(`"push_subscription".portal = ?`, iotModels.PushPortalParent).
			Scan(context.Background()))
		assert.Equal(t, guardian.ID, reboundCurrent.AccountID)
		assert.Equal(t, iotModels.PushPortalParent, reboundCurrent.Portal)

		deleted, err = repo.DeleteExpiredIfUnchanged(ctx, &reboundCurrent)
		require.NoError(t, err)
		assert.True(t, deleted)
	})

	t.Run("account-wide staff deletion preserves parent subscriptions", func(t *testing.T) {
		staffEndpoint := endpoint + "/logout-staff"
		parentEndpoint := endpoint + "/logout-parent"
		require.NoError(t, repo.Upsert(ctx, newSubscription(t, account.ID, iotModels.PushPortalStaff, staffEndpoint)))
		require.NoError(t, repo.Upsert(ctx, newSubscription(t, account.ID, iotModels.PushPortalParent, parentEndpoint)))

		require.NoError(t, tenant.WithAdminTx(context.Background(), db, func(adminCtx context.Context, _ bun.Tx) error {
			return repo.DeleteStaffByAccountID(adminCtx, account.ID)
		}))

		var remaining []*iotModels.PushSubscription
		require.NoError(t, db.NewSelect().
			Model(&remaining).
			ModelTableExpr(`iot.push_subscriptions AS "push_subscription"`).
			Where(`"push_subscription".account_id = ?`, account.ID).
			Scan(context.Background()))
		assert.NotEmpty(t, remaining)
		assert.True(t, hasSubscriptionEndpoint(remaining, parentEndpoint))
		for _, subscription := range remaining {
			assert.Equal(t, iotModels.PushPortalParent, subscription.Portal)
		}
	})

	t.Run("family deletion removes only that family's staff rows", func(t *testing.T) {
		keepEndpoint := endpoint + "/family-keep"
		dropEndpoint := endpoint + "/family-drop"
		keep := newSubscription(t, account.ID, iotModels.PushPortalStaff, keepEndpoint)
		keep.TokenFamilyID = "family-keep"
		drop := newSubscription(t, account.ID, iotModels.PushPortalStaff, dropEndpoint)
		drop.TokenFamilyID = "family-drop"
		require.NoError(t, repo.Upsert(ctx, keep))
		require.NoError(t, repo.Upsert(ctx, drop))

		require.NoError(t, tenant.WithAdminTx(context.Background(), db, func(adminCtx context.Context, _ bun.Tx) error {
			return repo.DeleteByTokenFamilyID(adminCtx, account.ID, "family-drop")
		}))

		var remaining []*iotModels.PushSubscription
		require.NoError(t, db.NewSelect().
			Model(&remaining).
			ModelTableExpr(`iot.push_subscriptions AS "push_subscription"`).
			Where(`"push_subscription".account_id = ?`, account.ID).
			Where(`"push_subscription".portal = ?`, iotModels.PushPortalStaff).
			Scan(context.Background()))
		assert.True(t, hasSubscriptionEndpoint(remaining, keepEndpoint))
		assert.False(t, hasSubscriptionEndpoint(remaining, dropEndpoint))
	})

	t.Run("unbound deletion is limited to the requested school", func(t *testing.T) {
		here := newSubscription(t, account.ID, iotModels.PushPortalStaff, endpoint+"/unbound-here")
		here.TokenFamilyID = ""
		otherSchool := createPushTestSchool(t, db)
		there := newSubscription(t, account.ID, iotModels.PushPortalStaff, endpoint+"/unbound-there")
		there.TokenFamilyID = ""
		there.SetTenantID(otherSchool.ID)
		require.NoError(t, repo.Upsert(ctx, here))
		require.NoError(t, tenant.WithAdminTx(context.Background(), db, func(adminCtx context.Context, _ bun.Tx) error {
			return repo.Upsert(adminCtx, there)
		}))

		require.NoError(t, tenant.WithAdminTx(context.Background(), db, func(adminCtx context.Context, _ bun.Tx) error {
			return repo.DeleteStaffUnboundByAccount(adminCtx, account.ID, tenant.FromContext(ctx))
		}))

		var remaining []*iotModels.PushSubscription
		require.NoError(t, db.NewSelect().
			Model(&remaining).
			ModelTableExpr(`iot.push_subscriptions AS "push_subscription"`).
			Where(`"push_subscription".account_id = ?`, account.ID).
			Where(`"push_subscription".token_family_id = ?`, "").
			Scan(context.Background()))
		assert.False(t, hasSubscriptionEndpoint(remaining, endpoint+"/unbound-here"))
		assert.True(t, hasSubscriptionEndpoint(remaining, endpoint+"/unbound-there"))
	})

	t.Run("family deletion removes parent rows for that family", func(t *testing.T) {
		keepEndpoint := endpoint + "/parent-family-keep"
		dropEndpoint := endpoint + "/parent-family-drop"
		keep := newSubscription(t, account.ID, iotModels.PushPortalParent, keepEndpoint)
		keep.TokenFamilyID = "parent-family-keep"
		drop := newSubscription(t, account.ID, iotModels.PushPortalParent, dropEndpoint)
		drop.TokenFamilyID = "parent-family-drop"
		require.NoError(t, repo.Upsert(ctx, keep))
		require.NoError(t, repo.Upsert(ctx, drop))

		require.NoError(t, tenant.WithAdminTx(context.Background(), db, func(adminCtx context.Context, _ bun.Tx) error {
			return repo.DeleteByTokenFamilyID(adminCtx, account.ID, "parent-family-drop")
		}))

		var remaining []*iotModels.PushSubscription
		require.NoError(t, db.NewSelect().
			Model(&remaining).
			ModelTableExpr(`iot.push_subscriptions AS "push_subscription"`).
			Where(`"push_subscription".account_id = ?`, account.ID).
			Where(`"push_subscription".portal = ?`, iotModels.PushPortalParent).
			Scan(context.Background()))
		assert.True(t, hasSubscriptionEndpoint(remaining, keepEndpoint))
		assert.False(t, hasSubscriptionEndpoint(remaining, dropEndpoint))
	})

	t.Run("parent unbound deletion is limited to the requested school", func(t *testing.T) {
		here := newSubscription(t, account.ID, iotModels.PushPortalParent, endpoint+"/parent-unbound-here")
		here.TokenFamilyID = ""
		otherSchool := createPushTestSchool(t, db)
		there := newSubscription(t, account.ID, iotModels.PushPortalParent, endpoint+"/parent-unbound-there")
		there.TokenFamilyID = ""
		there.SetTenantID(otherSchool.ID)
		require.NoError(t, repo.Upsert(ctx, here))
		require.NoError(t, tenant.WithAdminTx(context.Background(), db, func(adminCtx context.Context, _ bun.Tx) error {
			return repo.Upsert(adminCtx, there)
		}))

		require.NoError(t, tenant.WithAdminTx(context.Background(), db, func(adminCtx context.Context, _ bun.Tx) error {
			return repo.DeleteParentUnboundByAccount(adminCtx, account.ID, tenant.FromContext(ctx))
		}))

		var remaining []*iotModels.PushSubscription
		require.NoError(t, db.NewSelect().
			Model(&remaining).
			ModelTableExpr(`iot.push_subscriptions AS "push_subscription"`).
			Where(`"push_subscription".account_id = ?`, account.ID).
			Where(`"push_subscription".portal = ?`, iotModels.PushPortalParent).
			Where(`"push_subscription".token_family_id = ?`, "").
			Scan(context.Background()))
		assert.False(t, hasSubscriptionEndpoint(remaining, endpoint+"/parent-unbound-here"))
		assert.True(t, hasSubscriptionEndpoint(remaining, endpoint+"/parent-unbound-there"))
	})

	t.Run("guardian finder excludes subscriptions without a tenant mapping", func(t *testing.T) {
		// Pending-enrollment-only recipients have no mapping for the school.
		// Even a stale subscription row must therefore stay out of Web Push.
		_, err := db.NewDelete().
			TableExpr("auth.account_tenants").
			Where("account_id = ?", guardian.ID).
			Where("tenant_id = ?", tenant.FromContext(ctx)).
			Exec(context.Background())
		require.NoError(t, err)

		subs, err := repo.FindForGuardians(ctx, []int64{guardian.ID}, nil)
		require.NoError(t, err)
		assert.Empty(t, subs)
	})
}

func TestPushSubscriptionRepositoryEffectiveAdmins(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := iotRepo.NewPushSubscriptionRepository(db)
	ctx := testpkg.Ctx(t)
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
		createAccountTenantMapping(t, db, accountID, testpkg.Tenant(t))
		assignSystemRole(t, db, accountID, testpkg.Tenant(t), authModels.BaseRoleUser)
		require.NoError(t, repo.Upsert(ctx, newSubscription(t,
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
	directGrant.SetTenantID(testpkg.Tenant(t))
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
	roleGrant.SetTenantID(testpkg.Tenant(t))
	_, err = db.NewInsert().Model(roleGrant).ModelTableExpr("auth.account_roles").Exec(context.Background())
	require.NoError(t, err)

	subs, err := repo.FindForTenantAdmins(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, subscriptionsForAccount(subs, directAdmin.ID), "direct admin:* permission must grant admin push")
	assert.NotEmpty(t, subscriptionsForAccount(subs, roleAdmin.ID), "role-based *:* permission must grant admin push")
	assert.Empty(t, subscriptionsForAccount(subs, ordinary.ID), "ordinary staff must not receive admin push")

	var tenantRoleSubs []*iotModels.PushSubscription
	err = tenant.WithTenantTx(context.Background(), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		tenantRoleSubs, err = repo.FindForTenantAdmins(txCtx)
		return err
	})
	require.NoError(t, err)
	assert.NotEmpty(t, subscriptionsForAccount(tenantRoleSubs, directAdmin.ID))
	assert.NotEmpty(t, subscriptionsForAccount(tenantRoleSubs, roleAdmin.ID))
}
