// Guardian-portal consent, against a real database.
//
// The parents app carries no tenant of its own, so these paths resolve the
// account's active school mappings and act across all of them inside one admin
// transaction. That resolution, the RLS-guarded write it performs per school,
// and the tenant stamped on the row are exactly what a fake cannot check.
package notifications_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuardianPreferencesAcrossSchools(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	repos := repositories.NewFactory(db)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() {
		_, err := db.NewDelete().
			Table("users.notification_preferences").
			Where("account_id = ?", chain.AccountID).
			Exec(context.Background())
		require.NoError(t, err)
		testpkg.CleanupParentGuardianChain(t, db, chain)
	})

	settings := &configtest.Mock{
		ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
			return key == configModel.KeyNotificationsDispatchEnabled, nil
		},
	}
	svc := notifications.NewPreferenceService(repos.NotificationPreference, settings, db, repos.AccountTenant)

	t.Run("starts empty", func(t *testing.T) {
		overview, err := svc.GetForParent(ctx, chain.AccountID)
		require.NoError(t, err)
		require.Len(t, overview.Types, 1, "the guardian catalogue holds exactly one type today")
		assert.Equal(t, notifications.TypeParentAnnouncement, overview.Types[0].Key)
		assert.False(t, overview.Types[0].Enabled, "no row means off")
		assert.True(t, overview.Types[0].Available, "parent types carry no school-wide gate")
		assert.True(t, overview.TenantEnabled, "the family's school has dispatch on")
	})

	t.Run("a decision is stored per school and reads back", func(t *testing.T) {
		require.NoError(t, svc.SetForParent(ctx, chain.AccountID, notifications.TypeParentAnnouncement, true))

		overview, err := svc.GetForParent(ctx, chain.AccountID)
		require.NoError(t, err)
		assert.True(t, overview.Types[0].Enabled)

		// The row must carry the school it was decided for: without it the
		// tenant-scoped producer read would never find it again.
		var tenantID int64
		require.NoError(t, db.NewSelect().
			TableExpr("users.notification_preferences").
			ColumnExpr("tenant_id").
			Where("account_id = ? AND notification_type = ?", chain.AccountID, notifications.TypeParentAnnouncement).
			Scan(ctx, &tenantID))
		assert.Equal(t, chain.TenantID, tenantID)
	})

	t.Run("the producer finds the guardian through the tenant-scoped filter", func(t *testing.T) {
		tenantCtx := testpkg.TenantContext(chain.TenantID)
		optedIn, err := svc.FilterOptedIn(tenantCtx, notifications.TypeParentAnnouncement, []int64{chain.AccountID})
		require.NoError(t, err)
		assert.Equal(t, []int64{chain.AccountID}, optedIn,
			"what the parents app stored has to be what the announcement producer reads")
	})

	t.Run("a new school mapping keeps the shared switch off until consent covers it", func(t *testing.T) {
		const secondTenantID int64 = 2
		testpkg.EnsureTestTenant(t, db, secondTenantID)
		testpkg.MapAccountToTenant(t, db, chain.AccountID, secondTenantID)

		var guardianRoleID int64
		require.NoError(t, db.NewSelect().
			TableExpr("auth.roles").
			ColumnExpr("id").
			Where("name = ?", authModel.BaseRoleGuardian).
			Scan(ctx, &guardianRoleID))
		role := &authModel.AccountRole{AccountID: chain.AccountID, RoleID: guardianRoleID}
		role.SetTenantID(secondTenantID)
		_, err := db.NewInsert().Model(role).ModelTableExpr("auth.account_roles").Exec(ctx)
		require.NoError(t, err)

		overview, err := svc.GetForParent(ctx, chain.AccountID)
		require.NoError(t, err)
		assert.False(t, overview.Types[0].Enabled,
			"a missing consent row at the new school must not read as enabled")

		require.NoError(t, svc.SetForParent(ctx, chain.AccountID, notifications.TypeParentAnnouncement, true))
		overview, err = svc.GetForParent(ctx, chain.AccountID)
		require.NoError(t, err)
		assert.True(t, overview.Types[0].Enabled,
			"one shared change must record consent at every active school")
	})

	t.Run("switching everything off keeps the row as a deliberate no", func(t *testing.T) {
		require.NoError(t, svc.DisableAllForParent(ctx, chain.AccountID))

		overview, err := svc.GetForParent(ctx, chain.AccountID)
		require.NoError(t, err)
		assert.False(t, overview.Types[0].Enabled)

		var count int
		require.NoError(t, db.NewSelect().
			TableExpr("users.notification_preferences").
			ColumnExpr("count(*)").
			Where("account_id = ?", chain.AccountID).
			Scan(ctx, &count))
		assert.Equal(t, 2, count, "the opt-out is recorded once per active school, not deleted")

		tenantCtx := testpkg.TenantContext(chain.TenantID)
		optedIn, err := svc.FilterOptedIn(tenantCtx, notifications.TypeParentAnnouncement, []int64{chain.AccountID})
		require.NoError(t, err)
		assert.Empty(t, optedIn, "an opt-out reaches nobody")
	})

	t.Run("an unknown type is rejected before any write", func(t *testing.T) {
		err := svc.SetForParent(ctx, chain.AccountID, "not_a_type", true)
		require.ErrorIs(t, err, notifications.ErrUnknownNotificationType)
	})

	t.Run("an account with no school mapping changes nothing", func(t *testing.T) {
		orphan := testpkg.CreateTestAccount(t, db, "guardian-without-school@example.com")
		t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, orphan.ID) })

		require.NoError(t, svc.SetForParent(ctx, orphan.ID, notifications.TypeParentAnnouncement, true))

		var count int
		require.NoError(t, db.NewSelect().
			TableExpr("users.notification_preferences").
			ColumnExpr("count(*)").
			Where("account_id = ?", orphan.ID).
			Scan(ctx, &count))
		assert.Zero(t, count, "without a school there is nowhere to store a decision")

		overview, err := svc.GetForParent(ctx, orphan.ID)
		require.NoError(t, err)
		assert.False(t, overview.TenantEnabled, "no school means no school that enabled dispatch")
	})
}
