// Integration coverage for the production-wired branches of forEachTenant and
// forEachTenantSettings when the database, repository, and runtime are set.
package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// erroringSchoolRepo embeds the real repo and overrides only ListActive so the
// administrative tenant-listing transaction returns an error.
type erroringSchoolRepo struct {
	platform.SchoolRepository
	err error
}

func (e *erroringSchoolRepo) ListActive(_ context.Context) ([]platform.School, error) {
	return nil, e.err
}

func TestForEachTenant_Wired_InvokesFnPerTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	s := unitScheduler(&Scheduler{
		db:                      db,
		schoolRepo:              platformRepo.NewSchoolRepository(db),
		logger:                  slog.Default(),
		tenantRuntime:           testpkg.TenantRuntime(t, db),
		tenantRuntimeConfigured: true})

	var invocations int
	err := s.forEachTenant(context.Background(), "wired-iter", func(_ context.Context) error {
		invocations++
		return nil
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, invocations, 1,
		"wired iteration must invoke fn at least once for the seeded test tenant")
}

func TestForEachTenantSettings_Wired_InvokesFnWithTenantID(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	s := unitScheduler(&Scheduler{
		db:                      db,
		schoolRepo:              platformRepo.NewSchoolRepository(db),
		logger:                  slog.Default(),
		tenantRuntime:           testpkg.TenantRuntime(t, db),
		tenantRuntimeConfigured: true})

	var gotTenantID int64
	var invocations int
	s.forEachTenantSettings(context.Background(), "wired-settings", func(_ context.Context, tenantID int64) error {
		invocations++
		gotTenantID = tenantID
		return nil
	})

	assert.GreaterOrEqual(t, invocations, 1, "wired path must invoke fn for seeded tenant")
	assert.NotZero(t, gotTenantID, "wired path must supply the real tenant ID")
}

func TestForEachTenantSettings_Wired_ListerError_IsLogged(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	// Real repo provides the embedded type plus TxFromContext wiring; the
	// lister error comes from our override.
	s := unitScheduler(&Scheduler{
		db: db,
		schoolRepo: &erroringSchoolRepo{
			SchoolRepository: platformRepo.NewSchoolRepository(db),
			err:              errors.New("listing exploded"),
		},
		logger:                  slog.Default(),
		tenantRuntime:           testpkg.TenantRuntime(t, db),
		tenantRuntimeConfigured: true})

	var invoked bool
	s.forEachTenantSettings(context.Background(), "wired-err", func(_ context.Context, _ int64) error {
		invoked = true
		return nil
	})
	// The error path logs and returns — fn is never invoked because the
	// tenant list failed before iteration could start.
	assert.False(t, invoked, "fn must not run when the admin listing fails")
}

func TestForEachTenant_MissingRuntimeFailsClosed(t *testing.T) {
	t.Parallel()

	s := &Scheduler{logger: slog.Default()}
	called := false

	err := s.forEachTenant(context.Background(), "missing-runtime", func(context.Context) error {
		called = true
		return nil
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "tenant runtime is not configured")
	assert.False(t, called, "worker must not run without a tenant")
}

func TestForEachTenantSettings_MissingRuntimeSkipsWork(t *testing.T) {
	t.Parallel()

	s := &Scheduler{logger: slog.Default()}
	called := false

	completed := s.forEachTenantSettings(context.Background(), "missing-runtime", func(context.Context, int64) error {
		called = true
		return nil
	})

	assert.Empty(t, completed)
	assert.False(t, called, "worker must not run as tenant zero")
}

func TestForEachKnownTenantObservesMissingTenantAndTransactionFailure(t *testing.T) {
	t.Parallel()
	runtime, err := tenant.NewUnitOfWork(
		func(context.Context, int64, func(context.Context, any) error) error { return assert.AnError },
		func(ctx context.Context, fn func(context.Context, any) error) error { return fn(ctx, struct{}{}) },
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)

	var outcomes []string
	var unitOfWorkResults []string
	s := &Scheduler{tenantRuntime: runtime, tenantRuntimeConfigured: true}
	s.tenantRuntimeObserver = func(entryPoint, outcome string) {
		assert.Equal(t, "worker", entryPoint)
		outcomes = append(outcomes, outcome)
	}
	s.unitOfWorkObserver = func(entryPoint, kind, result string, _ time.Duration, _ int) {
		assert.Equal(t, "worker", entryPoint)
		assert.Equal(t, "transaction", kind)
		unitOfWorkResults = append(unitOfWorkResults, result)
	}

	s.forEachKnownTenant(context.Background(), []int64{0, 42}, "observe", func(context.Context, int64) error {
		t.Fatal("failed tenant transactions must not invoke the worker")
		return nil
	})

	assert.Equal(t, []string{"missing_tenant", "transaction_failure"}, outcomes)
	assert.Equal(t, []string{"rollback"}, unitOfWorkResults)
}
