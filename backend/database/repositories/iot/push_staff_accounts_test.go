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
)

// TestPushSubscriptionRepository_FindForStaffAccounts pins the addressing
// primitive for personal notifications: exactly the named recipients, and only
// while they are still eligible to receive anything.
func TestPushSubscriptionRepository_FindForStaffAccounts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := iotRepo.NewPushSubscriptionRepository(db)
	ctx := testpkg.Ctx(t)

	suffix := time.Now().UnixNano()
	addressed := testpkg.CreateTestAccount(t, db, fmt.Sprintf("push-addressed-%d@example.com", suffix))
	bystander := testpkg.CreateTestAccount(t, db, fmt.Sprintf("push-bystander-%d@example.com", suffix))

	defer testpkg.CleanupAuthFixtures(t, db, addressed.ID, bystander.ID)
	defer cleanupPushSubscriptions(t, db, addressed.ID, bystander.ID)

	createAccountTenantMapping(t, db, addressed.ID, testpkg.Tenant(t))
	createAccountTenantMapping(t, db, bystander.ID, testpkg.Tenant(t))
	assignSystemRole(t, db, addressed.ID, testpkg.Tenant(t), authModels.BaseRoleUser)
	assignSystemRole(t, db, bystander.ID, testpkg.Tenant(t), authModels.BaseRoleUser)

	addressedEndpoint := fmt.Sprintf("https://fcm.googleapis.com/fcm/send/addressed-%d", suffix)
	bystanderEndpoint := fmt.Sprintf("https://fcm.googleapis.com/fcm/send/bystander-%d", suffix)
	schoolEndpoint := fmt.Sprintf("https://fcm.googleapis.com/fcm/send/school-%d", suffix)

	require.NoError(t, repo.Upsert(ctx, newSubscription(t, addressed.ID, iotModels.PushPortalStaff, addressedEndpoint)))
	require.NoError(t, repo.Upsert(ctx, newSubscription(t, bystander.ID, iotModels.PushPortalStaff, bystanderEndpoint)))
	require.NoError(t, repo.Upsert(ctx, newSubscription(t, addressed.ID, iotModels.PushPortalSchool, schoolEndpoint)))

	t.Run("returns only the addressed account's devices", func(t *testing.T) {
		subs, err := repo.FindForStaffAccounts(ctx, []int64{addressed.ID})
		require.NoError(t, err)

		assert.True(t, hasSubscriptionEndpoint(subs, addressedEndpoint),
			"the named recipient's device must be returned")
		assert.Empty(t, subscriptionsForAccount(subs, bystander.ID),
			"a personal notification must not reach a bystander")
	})

	t.Run("keeps school devices separate from staff devices", func(t *testing.T) {
		staffSubs, err := repo.FindForStaffAccounts(ctx, []int64{addressed.ID})
		require.NoError(t, err)
		assert.True(t, hasSubscriptionEndpoint(staffSubs, addressedEndpoint))
		assert.False(t, hasSubscriptionEndpoint(staffSubs, schoolEndpoint))

		schoolSubs, err := repo.FindForSchoolAccounts(ctx, []int64{addressed.ID})
		require.NoError(t, err)
		assert.True(t, hasSubscriptionEndpoint(schoolSubs, schoolEndpoint))
		assert.False(t, hasSubscriptionEndpoint(schoolSubs, addressedEndpoint))
	})

	t.Run("empty input returns nothing rather than everything", func(t *testing.T) {
		// The dangerous default: an empty recipient list must never widen into
		// a tenant-wide send.
		subs, err := repo.FindForStaffAccounts(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, subs)
	})

	t.Run("agrees with the tenant-wide staff finder on eligibility", func(t *testing.T) {
		all, err := repo.FindForTenantStaff(ctx)
		require.NoError(t, err)

		narrowed, err := repo.FindForStaffAccounts(ctx, []int64{addressed.ID})
		require.NoError(t, err)

		assert.ElementsMatch(t,
			subscriptionsForAccount(all, addressed.ID),
			narrowed,
			"narrowing by account must not change which devices qualify")
	})

	t.Run("drops a device whose account was deactivated", func(t *testing.T) {
		_, err := db.NewUpdate().
			TableExpr("auth.accounts").
			Set("active = false").
			Where("id = ?", addressed.ID).
			Exec(ctx)
		require.NoError(t, err)

		defer func() {
			_, resetErr := db.NewUpdate().
				TableExpr("auth.accounts").
				Set("active = true").
				Where("id = ?", addressed.ID).
				Exec(ctx)
			require.NoError(t, resetErr)
		}()

		subs, err := repo.FindForStaffAccounts(ctx, []int64{addressed.ID})
		require.NoError(t, err)

		assert.Empty(t, subs,
			"eligibility is re-checked at delivery time, not inherited from the recipient list")
	})

	t.Run("drops a device whose tenant mapping is no longer active", func(t *testing.T) {
		_, err := db.NewUpdate().
			TableExpr("auth.account_tenants").
			Set("status = ?", authModels.AccountTenantStatusInactive).
			Where("account_id = ? AND tenant_id = ?", addressed.ID, testpkg.Tenant(t)).
			Exec(ctx)
		require.NoError(t, err)

		defer func() {
			_, resetErr := db.NewUpdate().
				TableExpr("auth.account_tenants").
				Set("status = ?", authModels.AccountTenantStatusActive).
				Where("account_id = ? AND tenant_id = ?", addressed.ID, testpkg.Tenant(t)).
				Exec(ctx)
			require.NoError(t, resetErr)
		}()

		subs, err := repo.FindForStaffAccounts(ctx, []int64{addressed.ID})
		require.NoError(t, err)

		assert.Empty(t, subs, "someone who left the school stops receiving its notifications")
	})

	t.Run("does not reach the same account from another tenant's context", func(t *testing.T) {
		otherTenantCtx := tenant.WithTenantID(context.Background(), 2)

		subs, err := repo.FindForStaffAccounts(otherTenantCtx, []int64{addressed.ID})
		require.NoError(t, err)

		assert.Empty(t, subs,
			"a device registered at one school must not receive another school's payload")
	})
}
