package compose

import (
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every role sentinel reaches consumers as its public twin with the same
// text; a wrapped sentinel keeps its message. The operation envelope is
// exercised end to end by the services/auth behaviour tests.
func TestRoleErrorTranslatesEverySentinel(t *testing.T) {
	t.Parallel()

	for _, sentinel := range roleSentinels {
		require.Equal(t, sentinel.internal.Error(), sentinel.public.Error(), "texts of %v", sentinel.public)
		assert.Same(t, sentinel.public, roleError(sentinel.internal))

		wrapped := roleError(fmt.Errorf("context: %w", sentinel.internal))
		require.ErrorIs(t, wrapped, sentinel.public)
		assert.Equal(t, "context: "+sentinel.internal.Error(), wrapped.Error())
	}
}

type storeFailure struct{ code string }

func (e storeFailure) Error() string { return "ERROR #" + e.code }

// Store failures keep their type so the RBAC routes still find constraint
// violations; the lifecycle and session sentinels an assignment shares keep
// their public twins.
func TestRoleErrorPassesUnknownErrorsThrough(t *testing.T) {
	t.Parallel()

	failure := storeFailure{code: "23503"}
	err := roleError(fmt.Errorf("delete role: %w", failure))
	var found storeFailure
	require.ErrorAs(t, err, &found)
	assert.Equal(t, "delete role: ERROR #23503", err.Error())

	other := errors.New("database is down")
	assert.Same(t, other, roleError(other))
	assert.NoError(t, roleError(nil))

	for _, sentinel := range lifecycleSentinels {
		assert.Same(t, sentinel.public, roleError(sentinel.internal))
	}
	for _, sentinel := range authenticationSentinels {
		assert.Same(t, sentinel.public, roleError(sentinel.internal))
	}
	nested := roleError(fmt.Errorf("provision school identity: %w", internalTwin(identityaccess.ErrSchoolIdentityTagTaken)))
	require.ErrorIs(t, nested, identityaccess.ErrSchoolIdentityTagTaken)
	assert.True(t, identityaccess.IsSchoolIdentityRequestError(nested))
}

// internalTwin returns the internal sentinel the lifecycle translation maps
// to public.
func internalTwin(public error) error {
	for _, sentinel := range lifecycleSentinels {
		if sentinel.public == public {
			return sentinel.internal
		}
	}
	return nil
}

func TestModuleWithoutLifecycleReportsRoleAdministrationUnavailable(t *testing.T) {
	t.Parallel()

	module := identityaccess.NewModule(engine{})
	ctx := t.Context()

	_, err := module.GetRole(ctx, 7)
	require.ErrorIs(t, err, identityaccess.ErrRoleAdministrationUnavailable)
	_, err = module.ListRoles(ctx, identityaccess.RoleFilter{})
	require.ErrorIs(t, err, identityaccess.ErrRoleAdministrationUnavailable)
	_, err = module.ResolveAssignableSchoolRole(ctx, 7, 8)
	require.ErrorIs(t, err, identityaccess.ErrRoleAdministrationUnavailable)
	_, err = module.AccountHoldsLehrkraftRole(ctx, 7)
	require.ErrorIs(t, err, identityaccess.ErrRoleAdministrationUnavailable)
	require.ErrorIs(t, module.AssignRoleToAccount(ctx, 7, 8), identityaccess.ErrRoleAdministrationUnavailable)
	require.ErrorIs(t, module.RemoveRoleFromAccount(ctx, 7, 8), identityaccess.ErrRoleAdministrationUnavailable)
	require.ErrorIs(t, module.ReplaceRolePermissions(ctx, 7, nil), identityaccess.ErrRoleAdministrationUnavailable)
	_, err = module.GetAccountPermissions(ctx, 7)
	require.ErrorIs(t, err, identityaccess.ErrRoleAdministrationUnavailable)
	_, err = module.HasLiveCaregiverProfile(ctx, 7)
	require.ErrorIs(t, err, identityaccess.ErrAccountLifecycleUnavailable)
	module.GrantStaffDefaultPermission(ctx, 7, false, "groups:read")
}
