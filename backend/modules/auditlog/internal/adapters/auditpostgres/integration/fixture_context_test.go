package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The Audit adapters read the tenant from their own handoff on the context.
// The shared fixture contexts carry it through the tenant runtime they are
// bound to, so the Audit rows a suite writes land in the test's tenant.
func TestFixtureContextsCarryTheAuditTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	scope := testpkg.NewTenantScope(t, db)
	assert.Equal(t, scope.TenantID, auditModels.TenantIDFromContext(scope.Context()))
	assert.Equal(t, testpkg.Tenant(t), auditModels.TenantIDFromContext(testpkg.Ctx(t)))
	assert.Equal(t, testpkg.OwnTenant(t), auditModels.TenantIDFromContext(testpkg.OwnCtx(t)))
}
