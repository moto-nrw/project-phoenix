package usercontext

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func contextWithRoles(userID int, roles ...string) context.Context {
	claims := jwt.AppClaims{
		ID:       userID,
		TenantID: 1,
		Roles:    roles,
	}
	ctx := tenant.WithTenantID(context.Background(), 1)
	return context.WithValue(ctx, jwt.CtxClaims, claims)
}

func TestIsAuthenticated(t *testing.T) {
	assert.True(t, isAuthenticated(contextWithRoles(42, "Admin")))
	assert.False(t, isAuthenticated(context.Background()))
}

func TestGetMyGroups_RejectsUnauthenticated(t *testing.T) {
	service := &userContextService{}

	groups, err := service.GetMyGroups(context.Background())
	require.Error(t, err)
	assert.Nil(t, groups)
}

func TestUserContextServiceGetLogger_FallsBackToDefault(t *testing.T) {
	service := &userContextService{}
	assert.NotNil(t, service.getLogger())
}
