package auth

import (
	"testing"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
)

func TestPersistedPortalScope(t *testing.T) {
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

func TestPushPortalForScope(t *testing.T) {
	assert.Equal(t, iotModels.PushPortalParent, pushPortalForScope(authModels.PortalScopeParent))
	assert.Equal(t, iotModels.PushPortalStaff, pushPortalForScope(authModels.PortalScopeTenant))
	assert.Equal(t, iotModels.PushPortalStaff, pushPortalForScope(authModels.PortalScopeOrg))
	assert.Equal(t, iotModels.PushPortalStaff, pushPortalForScope(authModels.PortalScopeSchool))
	assert.Equal(t, iotModels.PushPortalStaff, pushPortalForScope(authModels.PortalScopeUnknown))
	assert.Equal(t, iotModels.PushPortalStaff, pushPortalForScope(""))
}
