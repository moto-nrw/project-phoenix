package auth

import (
	"testing"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
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
