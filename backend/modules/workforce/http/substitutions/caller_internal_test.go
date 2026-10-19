package substitutions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func TestRequestCallerOnlyForThePrincipalsOwnTenant(t *testing.T) {
	t.Parallel()
	accountID, tenantID, otherTenantID := int64(41), int64(73), int64(74)
	principal, err := permissions.NewPrincipal(permissions.PrincipalInput{
		AccountID: accountID, TenantID: tenantID, Roles: []string{"staff"}, Permissions: []string{"substitutions:read"},
	})
	require.NoError(t, err)
	ctx := permissions.WithPrincipal(context.Background(), principal)

	caller, err := RequestCaller(tenant.WithTenantID(ctx, tenantID))
	require.NoError(t, err)
	assert.Equal(t, accountID, caller.AccountID)
	assert.Equal(t, tenantID, caller.TenantID)
	assert.Equal(t, []string{"staff"}, caller.Roles)
	assert.False(t, caller.Admin)
	assert.True(t, caller.HasPermission("substitutions:read"))

	_, err = RequestCaller(tenant.WithTenantID(ctx, otherTenantID))
	require.ErrorIs(t, err, workforce.ErrSubstitutionForbidden)
	_, err = RequestCaller(tenant.WithTenantID(context.Background(), tenantID))
	require.ErrorIs(t, err, workforce.ErrSubstitutionForbidden)
}
