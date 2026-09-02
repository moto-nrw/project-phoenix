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

	deleted, err := module.SoftDeleteOrganization(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, deleted.IsDeleted())
	persistedDeleted, err := module.FindOrganization(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, persistedDeleted.IsDeleted())

	restored, err := module.RestoreOrganization(ctx, created.ID)
	require.NoError(t, err)
	assert.False(t, restored.IsDeleted())
	persistedRestored, err := module.FindOrganization(ctx, created.ID)
	require.NoError(t, err)
	assert.False(t, persistedRestored.IsDeleted())
}

func TestModulePersistsSchoolLifecycleAtPublicSeam(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := adminContext(t)
	suffix := time.Now().UnixNano()

	organization, err := module.CreateOrganization(ctx, organizationtenancy.CreateOrganization{
		Name: fmt.Sprintf("School Organization %d", suffix), Slug: fmt.Sprintf("school-organization-%d", suffix), Active: true,
	})
	require.NoError(t, err)

	created, err := module.CreateSchool(ctx, organizationtenancy.CreateSchool{
		OrganizationID: organization.ID,
		Name:           fmt.Sprintf("School %d", suffix),
		Slug:           fmt.Sprintf("school-%d", suffix),
		Subdomain:      fmt.Sprintf("school-%d", suffix),
		Active:         true,
		Address:        "Testweg 1",
		City:           "Köln",
		Zip:            "50667",
		Email:          "schule@example.test",
	})
	require.NoError(t, err)
	assert.Positive(t, created.ID)

	found, err := module.FindSchool(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.Subdomain, found.Subdomain)
	assert.Equal(t, organization.ID, found.OrganizationID)

	listed, err := module.ListSchoolsByID(ctx, []int64{created.ID})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)

	updated, err := module.UpdateSchool(ctx, organizationtenancy.UpdateSchool{
		ID:             created.ID,
		OrganizationID: organization.ID,
		Name:           "Renamed School",
		Slug:           created.Slug,
		Subdomain:      created.Subdomain,
		Active:         false,
		Hidden:         true,
		Address:        created.Address,
		City:           created.City,
		Zip:            created.Zip,
		Email:          created.Email,
	})
	require.NoError(t, err)
	assert.Equal(t, "Renamed School", updated.Name)
	assert.False(t, updated.Active)
	assert.True(t, updated.Hidden)

	deleted, err := module.SoftDeleteSchool(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, deleted.IsDeleted())

	restored, err := module.RestoreSchool(ctx, created.ID)
	require.NoError(t, err)
	assert.False(t, restored.IsDeleted())
}

func TestSchoolLifecycleRejectsMissingOrDeletedOrganization(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := adminContext(t)
	suffix := time.Now().UnixNano()

	_, err := module.CreateSchool(ctx, organizationtenancy.CreateSchool{
		OrganizationID: 9_000_000_000 + suffix%100_000,
		Name:           "Orphan", Slug: fmt.Sprintf("orphan-%d", suffix), Subdomain: fmt.Sprintf("orphan-%d", suffix), Active: true,
	})
	require.ErrorIs(t, err, organizationtenancy.ErrOrganizationNotFound)

	organization, err := module.CreateOrganization(ctx, organizationtenancy.CreateOrganization{
		Name: fmt.Sprintf("Deleted Parent %d", suffix), Slug: fmt.Sprintf("deleted-parent-%d", suffix), Active: true,
	})
	require.NoError(t, err)
	_, err = module.SoftDeleteOrganization(ctx, organization.ID)
	require.NoError(t, err)

	_, err = module.CreateSchool(ctx, organizationtenancy.CreateSchool{
		OrganizationID: organization.ID,
		Name:           "Deleted Parent School", Slug: fmt.Sprintf("deleted-school-%d", suffix), Subdomain: fmt.Sprintf("deleted-school-%d", suffix), Active: true,
	})
	assert.ErrorIs(t, err, organizationtenancy.ErrOrganizationDeleted)
}

func TestModuleObservationsUsePublicErrors(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var observed Observation
	module := buildModule(t, db, func(observation Observation) { observed = observation })
	ctx := adminContext(t)
	slug := fmt.Sprintf("observation-organization-%d", time.Now().UnixNano())

	_, err := module.CreateOrganization(ctx, organizationtenancy.CreateOrganization{Name: "Observation", Slug: slug, Active: true})
	require.NoError(t, err)
	_, err = module.CreateOrganization(ctx, organizationtenancy.CreateOrganization{Name: "Duplicate", Slug: slug, Active: true})

	require.ErrorIs(t, err, organizationtenancy.ErrOrganizationSlugConflict)
	assert.ErrorIs(t, observed.Err, organizationtenancy.ErrOrganizationSlugConflict)
	assert.Equal(t, "slug_conflict", organizationtenancy.ErrorCode(observed.Err))
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

	_, err = module.ListSchools(ctx)
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

func TestSchoolTablePreservesTwoTenantRolePermissions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := adminContext(t)
	suffix := time.Now().UnixNano()
	organization, err := module.CreateOrganization(ctx, organizationtenancy.CreateOrganization{
		Name: fmt.Sprintf("School RLS Organization %d", suffix), Slug: fmt.Sprintf("school-rls-organization-%d", suffix), Active: true,
	})
	require.NoError(t, err)
	school, err := module.CreateSchool(ctx, organizationtenancy.CreateSchool{
		OrganizationID: organization.ID, Name: "School RLS", Slug: fmt.Sprintf("school-rls-%d", suffix),
		Subdomain: fmt.Sprintf("school-rls-%d", suffix), Active: true,
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
				return tx.NewSelect().TableExpr(`platform.schools AS "school"`).
					ColumnExpr(`"school".name`).Where(`"school".id = ?`, school.ID).Scan(txCtx, &name)
			})
			require.NoError(t, readErr, "tenant roles retain global school lookup access")

			writeErr := tenant.WithinTenant(tenantCtx, tenantID, func(txCtx context.Context) error {
				raw, ok := tenant.TransactionFromContext(txCtx)
				require.True(t, ok)
				tx := raw.(bun.Tx)
				_, updateErr := tx.NewUpdate().TableExpr(`platform.schools AS "school"`).
					Set("name = ?", "forbidden").Where(`"school".id = ?`, school.ID).Exec(txCtx)
				return updateErr
			})
			require.Error(t, writeErr, "tenant roles must not mutate platform.schools")
		})
	}
}
