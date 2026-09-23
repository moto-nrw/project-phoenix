package compose

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readChildQuota reads the Kinderkontingent the way School Membership does:
// inside a tenant transaction, for the tenant in context only.
func readChildQuota(t *testing.T, rawTenantID int64) (int, bool) {
	t.Helper()
	tenantID, err := tenant.NewTenantID(rawTenantID)
	require.NoError(t, err)
	var limit int
	var limited bool
	require.NoError(t, tenant.WithinTenant(testpkg.WithTestTenantRuntime(t, context.Background()), tenantID, func(txCtx context.Context) error {
		var readErr error
		limit, limited, readErr = NewChildQuotaLimits().ChildQuotaLimit(txCtx)
		return readErr
	}))
	return limit, limited
}

// TestSchoolChildQuotaIsStoredAndReadPerSchool pins the storage half of
// #3567: the operator sets, keeps and removes the Kinderkontingent; a school
// update leaves it alone; the tenant-safe read answers only for its school.
func TestSchoolChildQuotaIsStoredAndReadPerSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := adminContext(t)
	suffix := time.Now().UnixNano()
	organization, err := module.CreateOrganization(ctx, organizationtenancy.CreateOrganization{
		Name: fmt.Sprintf("Quota Organization %d", suffix), Slug: fmt.Sprintf("quota-organization-%d", suffix), Active: true,
	})
	require.NoError(t, err)
	school, err := module.CreateSchool(ctx, organizationtenancy.CreateSchool{
		OrganizationID: organization.ID, Name: "Quota School", Slug: fmt.Sprintf("quota-%d", suffix),
		Subdomain: fmt.Sprintf("quota-%d", suffix), Active: true,
	})
	require.NoError(t, err)
	assert.Nil(t, school.ChildQuota(), "a new school has no Kinderkontingent")
	assert.Equal(t, organizationtenancy.DefaultChildQuotaBundleSize, school.ChildQuotaBundleSize)
	_, limited := readChildQuota(t, school.ID)
	assert.False(t, limited)

	set, err := module.SetSchoolChildQuota(ctx, school.ID, &organizationtenancy.ChildQuota{Bundles: 2, BundleSize: 50})
	require.NoError(t, err)
	require.NotNil(t, set.ChildQuota())
	assert.Equal(t, 100, set.ChildQuota().Limit())

	updated, err := module.UpdateSchool(ctx, organizationtenancy.UpdateSchool{
		ID: school.ID, OrganizationID: organization.ID, Name: "Renamed Quota School",
		Slug: school.Slug, Subdomain: school.Subdomain, Active: true,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.ChildQuota(), "a school update keeps the contract value")
	assert.Equal(t, 100, updated.ChildQuota().Limit())

	limit, limited := readChildQuota(t, school.ID)
	assert.True(t, limited)
	assert.Equal(t, 100, limit)
	_, limited = readChildQuota(t, testpkg.Tenant(t))
	assert.False(t, limited, "another school does not see this Kinderkontingent")

	_, err = module.SetSchoolChildQuota(ctx, school.ID, &organizationtenancy.ChildQuota{Bundles: 0, BundleSize: 50})
	require.ErrorIs(t, err, organizationtenancy.ErrInvalidSchool)

	removed, err := module.SetSchoolChildQuota(ctx, school.ID, nil)
	require.NoError(t, err)
	assert.Nil(t, removed.ChildQuota())
	_, limited = readChildQuota(t, school.ID)
	assert.False(t, limited, "without a Kinderkontingent there is no limit")
}

func TestChildQuotaLimitsNeedATenantTransaction(t *testing.T) {
	t.Parallel()
	_, _, err := NewChildQuotaLimits().ChildQuotaLimit(context.Background())
	require.Error(t, err)
}
