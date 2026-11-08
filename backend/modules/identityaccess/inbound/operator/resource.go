// Package operator serves the operator identity routes of Identity & Access:
// operator login and refresh, the second factor, passkeys, invitations, the
// profile, the operator-led school access of accounts, the school-account
// MFA administration, and the operator scope and active-operator checks of
// the session chain (#3252, #3231). The operator router in api/operator
// mounts these handlers and middleware.
package operator

import (
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// Capability is the public Identity & Access capability the operator
// identity routes call.
type Capability interface {
	identityaccess.OperatorAuthentication
	identityaccess.OperatorAccountAccess
}

// Resource holds the operator identity handlers.
type Resource struct {
	identity Capability
	// accessFallback renders the school access outcomes the handlers do not
	// classify themselves; the root hands in the provisioning error mapping.
	accessFallback func(err error) render.Renderer
}

// NewResource binds the handlers to the composed capability.
func NewResource(identity Capability, accessFallback func(err error) render.Renderer) *Resource {
	return &Resource{identity: identity, accessFallback: accessFallback}
}
