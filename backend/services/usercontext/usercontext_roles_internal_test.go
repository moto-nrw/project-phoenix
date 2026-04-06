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

func TestCurrentUserHasRoleHelpers(t *testing.T) {
	ctx := contextWithRoles(42, "Admin", "User")

	assert.True(t, isAuthenticated(ctx))
	assert.True(t, currentUserHasRole(ctx, "admin"))
	assert.True(t, currentUserHasAnyRole(ctx, "teacher", "user"))
	assert.False(t, currentUserHasAnyRole(ctx, "guardian"))
	assert.False(t, isAuthenticated(context.Background()))
}

func TestCurrentUserHasAnyRole_AllowsExplicitAndLegacyRoleShapes(t *testing.T) {
	assert.True(t, currentUserHasAnyRole(contextWithRoles(42, "guardian"), "guardian"))
	assert.True(t, currentUserHasAnyRole(contextWithRoles(42), "user"))
}

func TestCaregiverRoleGate_AllowsExplicitTeacherRole(t *testing.T) {
	ctx := contextWithRoles(42, "teacher")

	assert.True(t, currentUserHasAnyRole(ctx, "user", "teacher", "admin"))
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
