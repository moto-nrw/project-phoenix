package usercontext

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func contextWithRoles(tb testing.TB, userID int, roles ...string) context.Context {
	claims := jwt.AppClaims{
		ID:       userID,
		TenantID: testpkg.Tenant(tb),
		Roles:    roles,
	}
	return context.WithValue(testpkg.Ctx(tb), jwt.CtxClaims, claims)
}

func TestIsAuthenticated(t *testing.T) {
	assert.True(t, isAuthenticated(contextWithRoles(t, 42, "Admin")))
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
