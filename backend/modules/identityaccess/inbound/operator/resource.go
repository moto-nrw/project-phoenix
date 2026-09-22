// Package operator serves the operator identity routes of Identity & Access:
// operator login and refresh, the second factor, passkeys, invitations, the
// profile, the operator-led school access of accounts, the school-account
// MFA administration, and the operator scope and active-operator checks of
// the session chain (#3252, #3231). The operator router in api/operator
// mounts these handlers and middleware and hands Resource its error bodies,
// so the wire format of the operator surface stays one.
package operator

import (
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// Capability is the public Identity & Access capability the operator
// identity routes call.
type Capability interface {
	identityaccess.OperatorAuthentication
	identityaccess.OperatorAccountAccess
}

// Responses are the operator surface's error bodies. OperatorResponses
// builds them from api/common; the router in api/operator supplies the
// school access fallback. The handlers here decide which body an outcome
// gets. The three fallbacks render every outcome the handlers do not
// classify themselves, such as the operator MFA errors a login can surface.
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

// OperatorResponses are the operator surface's error bodies from api/common.
// accessFallback renders the school access outcomes the handlers do not
// classify themselves; the root hands in the provisioning error mapping.
func OperatorResponses(accessFallback func(err error) render.Renderer) Responses {
	return Responses{
		InvalidRequest:     common.OperatorInvalidRequest,
		InvalidCredentials: common.OperatorInvalidCredentials,
		Unauthorized:       common.OperatorUnauthorized,
		NotFound:           common.OperatorNotFound,
		Forbidden:          common.OperatorForbidden,
		Conflict:           common.OperatorConflict,
		AuthFallback:       AuthErrorRenderer,
		ProfileFallback:    ProfileErrorRenderer,
		AccessFallback:     accessFallback,
	}
}
