package notifications_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPushSubscriptionServiceParentFiltersNonGuardianMappings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "push-parent-mixed-roles")
	guardianTenantID := testpkg.UniqueTestTenantID(t)
	staffTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, guardianTenantID)
	testpkg.EnsureTestTenant(t, db, staffTenantID)

	testpkg.MapAccountToTenant(t, db, account.ID, guardianTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, staffTenantID)

	var guardianRoleID, staffRoleID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("name = ?", "guardian").
		Scan(context.Background(), &guardianRoleID))
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("name = ?", "user").
		Scan(context.Background(), &staffRoleID))

	_, err := db.ExecContext(context.Background(), `
		INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
		VALUES (?, ?, ?), (?, ?, ?)`,
		account.ID, guardianRoleID, guardianTenantID,
		account.ID, staffRoleID, staffTenantID)
	require.NoError(t, err)

	service, err := services.NewPushSubscriptionTestService(db, testpkg.TenantRuntime(t, db),
		notifications.VAPIDConfig{PublicKey: "pub", PrivateKey: "priv", Subscriber: "mailto:test@example.org"})
	require.NoError(t, err)
	require.NoError(t, service.SubscribeParent(context.Background(), account.ID, notifications.PushSubscriptionInput{Endpoint: "https://fcm.googleapis.com/fcm/send/device", P256dh: "p256dh-key", Auth: "auth-key"}))
	var subscribed []int64
	require.NoError(t, db.NewRaw(`SELECT tenant_id FROM iot.push_subscriptions WHERE account_id = ? AND portal = 'parent'`, account.ID).Scan(context.Background(), &subscribed))
	assert.Equal(t, []int64{guardianTenantID}, subscribed)

	require.NoError(t, service.UnsubscribeParent(context.Background(), account.ID, notifications.PushSubscriptionInput{Endpoint: "https://fcm.googleapis.com/fcm/send/device", P256dh: "p256dh-key", Auth: "auth-key"}.Endpoint))
	count, err := db.NewSelect().TableExpr("iot.push_subscriptions").Where("account_id = ?", account.ID).Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count)
}
