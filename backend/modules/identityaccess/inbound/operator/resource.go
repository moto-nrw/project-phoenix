// Package operator serves the operator identity routes of Identity & Access
// (#3252): operator login, refresh, the profile and password changes, and
// the operator-led school access of accounts. The operator router in
// api/operator mounts these handlers behind its middleware chain and hands
// in its error bodies, so the wire format of the operator surface stays one.
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

// Responses are the operator surface's error bodies. The operator router
// owns their shape; the handlers here decide which one an outcome gets. The
// three fallbacks render every outcome the handlers do not classify
// themselves, such as the retained operator MFA errors a login can surface.
type Responses struct {
	InvalidRequest     func(err error) render.Renderer
	InvalidCredentials func() render.Renderer
	Unauthorized       func() render.Renderer
	NotFound           func(message string) render.Renderer
	Forbidden          func(message string) render.Renderer
	Conflict           func(message string) render.Renderer
	AuthFallback       func(err error) render.Renderer
	ProfileFallback    func(err error) render.Renderer
	AccessFallback     func(err error) render.Renderer
}

// Resource holds the operator identity handlers.
type Resource struct {
	identity  Capability
	responses Responses
}

// NewResource binds the handlers to the composed capability and the
// operator surface's error bodies.
func NewResource(identity Capability, responses Responses) *Resource {
	return &Resource{identity: identity, responses: responses}
}
