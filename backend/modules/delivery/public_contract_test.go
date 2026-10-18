package delivery_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type emptyGuardianNames struct{}

func (emptyGuardianNames) ResolveGuardianDisplays(context.Context, []int64) ([]delivery.GuardianDisplay, error) {
	return nil, nil
}
func TestPublicDeliveryContractDeduplicatesDurableIntent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	runtime, err := compose.NewModule(compose.ModuleDependencies{DB: db, People: emptyGuardianNames{}, Observe: func(delivery.Observation) {}})
	require.NoError(t, err)
	_, account := testpkg.CreateTestPersonWithAccount(t, db, "Delivery", "Recipient")
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		intent := delivery.EmailIntent{TenantID: testpkg.Tenant(t), Template: "contract", Recipient: delivery.EmailRecipient{Address: account.Email}, Payload: json.RawMessage(`{"subject":"Contract"}`), IdempotencyKey: "contract-email", Related: delivery.RelatedEntity{Type: "contract", ID: account.ID}}
		first, err := runtime.Module.EnqueueEmail(ctx, intent)
		require.NoError(t, err)
		require.Positive(t, first.ID)
		require.False(t, first.Duplicate)
		second, err := runtime.Module.EnqueueEmail(ctx, intent)
		require.NoError(t, err)
		require.Equal(t, first.ID, second.ID)
		require.True(t, second.Duplicate)
		rows, err := runtime.Module.Statuses(ctx, intent.TenantID, delivery.TransportEmail, intent.Related)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, first.ID, rows[0].ID)
		require.Equal(t, delivery.StatePending, rows[0].State)
		return nil
	}))
}
