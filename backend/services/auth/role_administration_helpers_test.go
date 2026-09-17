package auth_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/stretchr/testify/require"
)

// roleAdministrationOf returns the Identity & Access module the retained auth
// service was composed with. Role and permission management moved there
// (#3314); the behaviour tests drive its public contract.
func roleAdministrationOf(t *testing.T, service auth.AuthService) *identityaccess.Module {
	t.Helper()
	if owned, ok := service.(*fixtureOwnedAuthService); ok {
		service = owned.AuthService
	}
	roles := services.IdentityAccessForTests(service)
	require.NotNil(t, roles, "the auth service must be composed with Identity & Access")
	return roles
}
