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

func TestGetCurrentStaff_RejectsExplicitWrongRoleBeforeRepoLookup(t *testing.T) {
	service := &userContextService{}

	staff, err := service.GetCurrentStaff(contextWithRoles(42, "guardian"))

	require.Error(t, err)
	assert.Nil(t, staff)
	assert.ErrorIs(t, err, ErrUserNotLinkedToStaff)
}

func TestGetCurrentTeacher_RejectsExplicitWrongRoleBeforeRepoLookup(t *testing.T) {
	service := &userContextService{}

	teacher, err := service.GetCurrentTeacher(contextWithRoles(42, "guardian"))

	require.Error(t, err)
	assert.Nil(t, teacher)
	assert.ErrorIs(t, err, ErrUserNotLinkedToTeacher)
}

func TestCaregiverRoleGate_AllowsExplicitTeacherRole(t *testing.T) {
	ctx := contextWithRoles(42, "teacher")

	assert.True(t, currentUserHasAnyRole(ctx, "user", "teacher", "admin"))
}

func TestGetMyGroups_RejectsUnauthenticatedAndAllowsNonCaregiverFallback(t *testing.T) {
	service := &userContextService{}

	groups, err := service.GetMyGroups(context.Background())
	require.Error(t, err)
	assert.Nil(t, groups)

	groups, err = service.GetMyGroups(contextWithRoles(42, "guardian"))
	require.NoError(t, err)
	assert.Empty(t, groups)
}

func TestUserContextServiceGetLogger_FallsBackToDefault(t *testing.T) {
	service := &userContextService{}
	assert.NotNil(t, service.getLogger())
}
