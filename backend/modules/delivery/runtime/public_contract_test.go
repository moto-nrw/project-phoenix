package delivery_test

import (
	"context"
	"encoding/json"
	"testing"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	"github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type emptyGuardianNames struct{}

func (emptyGuardianNames) ResolveGuardianDisplays(context.Context, []int64) ([]deliveryContract.GuardianDisplay, error) {
	return nil, nil
}
func TestPublicDeliveryContractDeduplicatesDurableIntent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	runtime, err := compose.NewModule(compose.ModuleDependencies{DB: db, People: emptyGuardianNames{}, Observe: func(deliveryContract.Observation) {}})
	require.NoError(t, err)
	_, account := testpkg.CreateTestPersonWithAccount(t, db, "Delivery", "Recipient")
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		intent := deliveryContract.EmailIntent{TenantID: testpkg.Tenant(t), Template: "contract", Recipient: deliveryContract.EmailRecipient{Address: account.Email}, Payload: json.RawMessage(`{"subject":"Contract"}`), IdempotencyKey: "contract-email", Related: deliveryContract.RelatedEntity{Type: "contract", ID: account.ID}}
		first, err := runtime.Module.EnqueueEmail(ctx, intent)
		require.NoError(t, err)
		require.Positive(t, first.ID)
		require.False(t, first.Duplicate)
		second, err := runtime.Module.EnqueueEmail(ctx, intent)
		require.NoError(t, err)
		require.Equal(t, first.ID, second.ID)
		require.True(t, second.Duplicate)
		rows, err := runtime.Module.Statuses(ctx, intent.TenantID, deliveryContract.TransportEmail, intent.Related)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, first.ID, rows[0].ID)
		require.Equal(t, deliveryContract.StatePending, rows[0].State)
		return nil
	}))
}
