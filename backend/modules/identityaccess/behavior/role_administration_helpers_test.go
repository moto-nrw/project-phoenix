package behavior_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/require"
)

// roleAdministrationOf returns the Identity & Access module the fixture was
// composed with. Role and permission management belong to the module
// (#3314); the behaviour tests drive its public contract.
func roleAdministrationOf(t *testing.T, service testAuthService) *identityaccess.Module {
	t.Helper()
	owned, ok := service.(*fixtureOwnedAuthService)
	require.True(t, ok, "the auth fixture must be composed with Identity & Access")
	require.NotNil(t, owned.module, "the auth fixture must be composed with Identity & Access")
	return owned.module
}
