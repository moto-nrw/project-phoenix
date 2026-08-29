package auth

import (
	"testing"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
)

func TestPersistedPortalScope(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"":                     authModels.PortalScopeTenant,
		tenant.ScopeOrg:        authModels.PortalScopeOrg,
		tenant.ScopeParent:     authModels.PortalScopeParent,
		tenant.ScopeSchool:     authModels.PortalScopeSchool,
		"unexpected-new-scope": authModels.PortalScopeUnknown,
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			assert.Equal(t, want, persistedPortalScope(input))
		})
	}
}

func TestPushPortalsForScope(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{iotModels.PushPortalParent}, pushPortalsForScope(authModels.PortalScopeParent))
	assert.Equal(t, []string{iotModels.PushPortalStaff}, pushPortalsForScope(authModels.PortalScopeTenant))
	assert.Equal(t, []string{iotModels.PushPortalStaff}, pushPortalsForScope(authModels.PortalScopeOrg))
	// A school logout revokes school devices only (#2208): the same account's
	// OGS devices keep their registration.
	assert.Equal(t, []string{iotModels.PushPortalSchool}, pushPortalsForScope(authModels.PortalScopeSchool))
	assert.Equal(t, []string{iotModels.PushPortalStaff, iotModels.PushPortalParent, iotModels.PushPortalSchool}, pushPortalsForScope(authModels.PortalScopeUnknown))
	assert.Equal(t, []string{iotModels.PushPortalStaff, iotModels.PushPortalParent, iotModels.PushPortalSchool}, pushPortalsForScope(""))
}
