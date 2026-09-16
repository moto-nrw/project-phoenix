package operator

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// authError maps login and refresh outcomes. An unknown or foreign operator
// is indistinguishable from wrong credentials.
func (rs *Resource) authError(err error) render.Renderer {
	switch {
	case errors.Is(err, identityaccess.ErrOperatorRefreshTokenInvalid):
		return rs.responses.Unauthorized()
	case errors.Is(err, identityaccess.ErrOperatorInvalidCredentials),
		errors.Is(err, identityaccess.ErrOperatorNotFound):
		return rs.responses.InvalidCredentials()
	case errors.Is(err, identityaccess.ErrOperatorInactive):
		return rs.responses.Forbidden("Operator account is inactive")
	default:
		return rs.responses.AuthFallback(err)
	}
}

// profileError maps profile and password change outcomes.
func (rs *Resource) profileError(err error) render.Renderer {
	if _, ok := errors.AsType[*identityaccess.InvalidInputError](err); ok {
		return rs.responses.InvalidRequest(err)
	}
	switch {
	case errors.Is(err, identityaccess.ErrOperatorPasswordMismatch):
		return rs.responses.InvalidRequest(errors.New("das aktuelle Passwort ist falsch"))
	case errors.Is(err, identityaccess.ErrOperatorNotFound):
		return rs.responses.NotFound("Operator not found")
	case errors.Is(err, identityaccess.ErrOperatorInactive):
		return rs.responses.Forbidden("Dieser Account ist deaktiviert")
	default:
		return rs.responses.ProfileFallback(err)
	}
}

// accessError surfaces validation messages verbatim: the operator needs to
// know why a role was rejected.
func (rs *Resource) accessError(err error) render.Renderer {
	if invalid, ok := errors.AsType[*identityaccess.InvalidInputError](err); ok {
		return rs.responses.InvalidRequest(invalid.Err)
	}
	switch {
	case errors.Is(err, identityaccess.ErrAccountNotFound):
		return rs.responses.NotFound("Account not found")
	case errors.Is(err, identityaccess.ErrAccountTenantAccessNotFound):
		return rs.responses.NotFound("Account has no access to this school")
	case errors.Is(err, identityaccess.ErrAccountTenantAccessExists):
		return rs.responses.Conflict("account already has access to this school")
	case errors.Is(err, identityaccess.ErrSchoolNotFound):
		return rs.responses.NotFound("School not found")
	case errors.Is(err, identityaccess.ErrSchoolDeleted):
		return rs.responses.Conflict("School is already deleted")
	default:
		return rs.responses.AccessFallback(err)
	}
}
