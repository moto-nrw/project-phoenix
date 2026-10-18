package delivery_test

import (
	"context"
	"fmt"
	"testing"

	deliveryRepo "github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/deliverypostgres"
	deliveryModels "github.com/moto-nrw/project-phoenix/modules/delivery/subscriptions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestDeleteUnboundByAccountForPortalDeletesOnlyThatPortal(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repo := &deliveryRepo.PushSubscriptionRepository{DB: db}
	tenantID := testpkg.Tenant(t)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("push-portal-%d@test.local", tenantID))
	for _, portal := range []string{deliveryModels.PushPortalStaff, deliveryModels.PushPortalParent, deliveryModels.PushPortalSchool} {
		require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
			return repo.RegisterSubscription(ctx, &deliveryModels.PushSubscription{
				AccountID: account.ID, Portal: portal, Endpoint: fmt.Sprintf("https://fcm.googleapis.com/fcm/send/%s-%d", portal, tenantID),
				P256dh: "key", Auth: "auth",
			})
		}))
	}
	remaining := func() []string {
		var portals []string
		require.NoError(t, testpkg.WithAdminTx(t, testpkg.Ctx(t), db, func(ctx context.Context, tx bun.Tx) error {
			return tx.NewSelect().TableExpr("iot.push_subscriptions").ColumnExpr("portal").
				Where("account_id = ?", account.ID).OrderExpr("portal").Scan(ctx, &portals)
		}))
		return portals
	}
	deleteFor := func(portal string) {
		require.NoError(t, testpkg.WithAdminTx(t, testpkg.Ctx(t), db, func(ctx context.Context, _ bun.Tx) error {
			return repo.DeleteUnboundByAccountForPortal(ctx, account.ID, tenantID, portal)
		}))
	}

	deleteFor(deliveryModels.PushPortalSchool)
	assert.Equal(t, []string{deliveryModels.PushPortalParent, deliveryModels.PushPortalStaff}, remaining())
	deleteFor("unknown")
	assert.Equal(t, []string{deliveryModels.PushPortalParent}, remaining(), "an unknown portal is the staff portal")
	deleteFor(deliveryModels.PushPortalParent)
	assert.Empty(t, remaining())
}
