package compose

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	applicationBindings "github.com/moto-nrw/project-phoenix/modules/delivery/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

func TestProviderResultMapsPublicCancellationToDomainDecision(t *testing.T) {
	t.Parallel()
	_, err := applicationBindings.ProviderResult(deliveryContract.ProviderResult{}, fmt.Errorf("access revoked: %w", deliveryContract.ErrCancelled))
	assert.True(t, errors.Is(err, domain.ErrCancelled))
}

type acceptingProvider struct{}

func (acceptingProvider) SendEmail(context.Context, deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error) {
	return deliveryContract.ProviderResult{}, nil
}

func (acceptingProvider) SendPush(context.Context, deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error) {
	return deliveryContract.ProviderResult{}, nil
}

type noGuardians struct{}

func (noGuardians) ResolveGuardianDisplays(context.Context, []int64) ([]deliveryContract.GuardianDisplay, error) {
	return nil, nil
}

func TestNewModuleAndWorkerRequireTheirDependencies(t *testing.T) {
	t.Parallel()
	// The connector opens no connection until a query runs.
	db := bun.NewDB(sql.OpenDB(pgdriver.NewConnector()), pgdialect.New())
	observe := func(deliveryContract.Observation) {}

	for name, dependencies := range map[string]ModuleDependencies{
		"database": {People: noGuardians{}, Observe: observe},
		"people":   {DB: db, Observe: observe},
		"observer": {DB: db, People: noGuardians{}},
	} {
		_, err := NewModule(dependencies)
		require.Error(t, err, "missing %s", name)
	}

	runtime, err := NewModule(ModuleDependencies{DB: db, People: noGuardians{}, Observe: observe})
	require.NoError(t, err)
	require.NotNil(t, runtime.Module)

	_, err = NewWorker(nil, acceptingProvider{})
	require.Error(t, err)
	_, err = NewWorker(&Runtime{}, acceptingProvider{})
	require.Error(t, err)
	_, err = NewWorker(runtime, nil)
	require.Error(t, err)
	worker, err := NewWorker(runtime, acceptingProvider{})
	require.NoError(t, err)
	assert.NotNil(t, worker)
}

func TestNewComposesModuleAndWorker(t *testing.T) {
	t.Parallel()
	db := bun.NewDB(sql.OpenDB(pgdriver.NewConnector()), pgdialect.New())
	runtime, err := NewModule(ModuleDependencies{DB: db, People: noGuardians{}, Observe: func(deliveryContract.Observation) {}})
	require.NoError(t, err)
	_, err = NewWorker(runtime, nil)
	require.Error(t, err, "the provider is required")

	worker, err := NewWorker(runtime, acceptingProvider{})
	require.NoError(t, err)
	assert.NotNil(t, runtime.Module)
	assert.NotNil(t, worker)
	result, err := runtime.Module.EnqueueEmail(context.Background(), deliveryContract.EmailIntent{})
	require.EqualError(t, err, "delivery: tenant is required")
	assert.Equal(t, deliveryContract.Enqueued{}, result)
	processed, err := worker.RunOnce(context.Background(), 0, 3)
	require.EqualError(t, err, "delivery worker: batch size must be positive")
	assert.Zero(t, processed)
}
