package compose

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func buildModule(t *testing.T, db *bun.DB, observations ...func(Observation)) *organizationtenancy.Module {
	t.Helper()
	observe := func(Observation) {}
	if len(observations) > 0 {
		observe = observations[0]
	}
	module, err := New(Dependencies{DB: db, Observe: observe})
	require.NoError(t, err)
	return module
}

func adminContext(t *testing.T) context.Context {
	t.Helper()
	return testpkg.WithTestTenantRuntime(t, context.Background())
}

func TestModulePersistsOrganizationLifecycleAtPublicSeam(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := adminContext(t)
	suffix := time.Now().UnixNano()

	created, err := module.CreateOrganization(ctx, organizationtenancy.CreateOrganization{
		Name: fmt.Sprintf("Organization %d", suffix), Slug: fmt.Sprintf("organization-%d", suffix), Active: true,
	})
	require.NoError(t, err)
	assert.Positive(t, created.ID)

	found, err := module.FindOrganization(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.Slug, found.Slug)

	listed, err := module.ListOrganizationsByID(ctx, []int64{created.ID})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)
	count, err := module.CountOrganizationsByID(ctx, []int64{created.ID, created.ID + 999999})
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	updated, err := module.UpdateOrganization(ctx, organizationtenancy.UpdateOrganization{
		ID: created.ID, Name: "Renamed Organization", Slug: created.Slug, Active: false,
	})
	require.NoError(t, err)
	assert.Equal(t, "Renamed Organization", updated.Name)
	assert.False(t, updated.Active)

	_, err = module.SoftDeleteOrganization(ctx, created.ID)
	require.NoError(t, err)
	deleted, err := module.FindOrganization(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, deleted.IsDeleted())

	_, err = module.RestoreOrganization(ctx, created.ID)
	require.NoError(t, err)
	restored, err := module.FindOrganization(ctx, created.ID)
	require.NoError(t, err)
	assert.False(t, restored.IsDeleted())
}

func TestModuleCommandRollsBackWithOuterUnitOfWork(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := adminContext(t)
	slug := fmt.Sprintf("rollback-organization-%d", time.Now().UnixNano())
	wantErr := errors.New("abort outer command")

	err := tenant.WithinAdmin(ctx, func(txCtx context.Context) error {
		_, createErr := module.CreateOrganization(txCtx, organizationtenancy.CreateOrganization{Name: "Rollback", Slug: slug, Active: true})
		require.NoError(t, createErr)
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	_, err = module.FindOrganizationBySlug(ctx, slug)
	assert.ErrorIs(t, err, organizationtenancy.ErrOrganizationNotFound)
}

func TestModuleKeepsPersistenceFailuresVisible(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx, cancel := context.WithCancel(adminContext(t))
	cancel()

	_, err := module.ListOrganizations(ctx)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestOrganizationTableDeniesTwoTenantRoles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := adminContext(t)
	created, err := module.CreateOrganization(ctx, organizationtenancy.CreateOrganization{
		Name: "RLS Organization", Slug: fmt.Sprintf("rls-organization-%d", time.Now().UnixNano()), Active: true,
	})
	require.NoError(t, err)

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	for _, rawTenantID := range []int64{testpkg.Tenant(t), otherTenantID} {
		tenantID, tenantErr := tenant.NewTenantID(rawTenantID)
		require.NoError(t, tenantErr)
		t.Run(fmt.Sprintf("tenant-%d", rawTenantID), func(t *testing.T) {
			tenantCtx := testpkg.WithTestTenantRuntime(t, context.Background())
			readErr := tenant.WithinTenant(tenantCtx, tenantID, func(txCtx context.Context) error {
				raw, ok := tenant.TransactionFromContext(txCtx)
				require.True(t, ok)
				tx := raw.(bun.Tx)
				var name string
				return tx.NewSelect().TableExpr(`platform.organizations AS "organization"`).
					ColumnExpr(`"organization".name`).Where(`"organization".id = ?`, created.ID).Scan(txCtx, &name)
			})
			require.Error(t, readErr, "tenant role must not read platform.organizations")

			writeErr := tenant.WithinTenant(tenantCtx, tenantID, func(txCtx context.Context) error {
				raw, ok := tenant.TransactionFromContext(txCtx)
				require.True(t, ok)
				tx := raw.(bun.Tx)
				_, updateErr := tx.NewUpdate().TableExpr(`platform.organizations AS "organization"`).
					Set("name = ?", "forbidden").Where(`"organization".id = ?`, created.ID).Exec(txCtx)
				return updateErr
			})
			require.Error(t, writeErr, "tenant role must not mutate platform.organizations")
		})
	}
}
