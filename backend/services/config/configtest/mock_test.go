package configtest_test

import (
	"context"
	"errors"
	"testing"

	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockBatchResolversDelegateAndDefault(t *testing.T) {
	ctx := context.Background()
	keys := []string{"test.key"}
	var tenantID int64
	tenantIDs := []int64{tenantID}
	delegateErr := errors.New("delegate called")

	mock := &configtest.Mock{
		ResolveManyForTenantFn: func(
			gotCtx context.Context,
			gotTenantID int64,
			gotKeys []string,
		) (*configService.SettingsSnapshot, error) {
			assert.Equal(t, ctx, gotCtx)
			assert.Equal(t, tenantID, gotTenantID)
			assert.Equal(t, keys, gotKeys)
			return nil, delegateErr
		},
		ResolveManyForTenantsFn: func(
			gotCtx context.Context,
			gotTenantIDs []int64,
			gotKeys []string,
		) (map[int64]*configService.SettingsSnapshot, error) {
			assert.Equal(t, ctx, gotCtx)
			assert.Equal(t, tenantIDs, gotTenantIDs)
			assert.Equal(t, keys, gotKeys)
			return nil, delegateErr
		},
	}

	_, err := mock.ResolveManyForTenant(ctx, tenantID, keys)
	require.ErrorIs(t, err, delegateErr)
	_, err = mock.ResolveManyForTenants(ctx, tenantIDs, keys)
	require.ErrorIs(t, err, delegateErr)

	result, err := (&configtest.Mock{}).ResolveManyForTenants(ctx, tenantIDs, keys)
	require.NoError(t, err)
	assert.Nil(t, result)
}
