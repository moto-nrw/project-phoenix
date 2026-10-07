package operator

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
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
		return operatorInactive("Operator account is inactive")
	default:
		return rs.responses.AuthFallback(err)
	}
}

// profileError maps profile and password change outcomes.
func (rs *Resource) profileError(err error) render.Renderer {
	if invalid, ok := errors.AsType[*identityaccess.InvalidInputError](err); ok {
		return invalidOperatorInput(invalid.Err, "new_password")
	}
	switch {
	case errors.Is(err, identityaccess.ErrOperatorPasswordMismatch):
		return common.OperatorInvalidField(common.CodeIdentityCurrentPasswordWrong, "current_password", "das aktuelle Passwort ist falsch")
	case errors.Is(err, identityaccess.ErrOperatorNotFound):
		return common.OperatorRejection(http.StatusNotFound, common.CodeIdentityAccountNotFound, "Operator not found")
	case errors.Is(err, identityaccess.ErrOperatorInactive):
		return operatorInactive("Dieser Account ist deaktiviert")
	default:
		return rs.responses.ProfileFallback(err)
	}
}

// accessError surfaces validation messages verbatim: the operator needs to
// know why a role was rejected.
func (rs *Resource) accessError(err error) render.Renderer {
	if invalid, ok := errors.AsType[*identityaccess.InvalidInputError](err); ok {
		if errors.Is(invalid.Err, identityaccess.ErrLehrkraftRoleImmutable) {
			return common.OperatorRejection(http.StatusBadRequest, common.CodeIdentityLehrkraftRoleImmutable, invalid.Err.Error())
		}
		return rs.responses.InvalidRequest(invalid.Err)
	}
	switch {
	case errors.Is(err, identityaccess.ErrAccountNotFound):
		return common.OperatorRejection(http.StatusNotFound, common.CodeIdentityAccountNotFound, "Account not found")
	case errors.Is(err, identityaccess.ErrAccountTenantAccessNotFound):
		return common.OperatorRejection(http.StatusNotFound, common.CodeIdentityTenantAccessNotFound, "Account has no access to this school")
	case errors.Is(err, identityaccess.ErrAccountTenantAccessExists):
		return common.OperatorRejection(http.StatusConflict, common.CodeIdentityAccountAlreadyHasTenantAccess, "account already has access to this school")
	case errors.Is(err, identityaccess.ErrSchoolNotFound):
		return common.OperatorRejection(http.StatusNotFound, common.CodeProvisioningSchoolNotFound, "School not found")
	case errors.Is(err, identityaccess.ErrSchoolDeleted):
		return common.OperatorRejection(http.StatusConflict, common.CodeProvisioningSchoolAlreadyDeleted, "School is already deleted")
	default:
		return rs.responses.AccessFallback(err)
	}
}

// AuthErrorRenderer maps Identity & Access authentication outcomes to HTTP
// responses. The bodies are fixed sentences, so the owner's sentinels decide
// exactly the statuses and texts the retained typed errors decided (#3364).
func AuthErrorRenderer(err error) render.Renderer {
	switch {
	case errors.Is(err, ErrOperatorInvalidCredentials):
		return common.OperatorInvalidCredentials()
	case errors.Is(err, ErrOperatorInactive):
		return operatorInactive("Operator account is inactive")
	case errors.Is(err, ErrOperatorNotFound):
		return common.OperatorInvalidCredentials()
	case errors.Is(err, ErrMFARateLimited):
		return common.OperatorRejection(http.StatusTooManyRequests, common.CodeIdentityMfaBlocked, "Too many code requests, please wait")
	case errors.Is(err, ErrMFALocked):
		return common.OperatorRejection(http.StatusTooManyRequests, common.CodeIdentityMfaBlocked, "MFA account temporarily locked")
	case errors.Is(err, ErrMFAChallengeTokenInvalid),
		errors.Is(err, ErrMFACodeInvalid):
		return common.OperatorRejection(http.StatusUnauthorized, common.CodeIdentityMfaCodeInvalid, "Invalid email or password")
	case errors.Is(err, ErrMFAStatusUnavailable):
		return common.OperatorServiceUnavailable("MFA status temporarily unavailable, please retry")
	default:
		return common.OperatorInternal("Authentication failed")
	}
}

// operatorInactive answers a deactivated operator account: 403 with the
// registered inactive-account code (#2519).
func operatorInactive(message string) render.Renderer {
	return common.OperatorRejection(http.StatusForbidden, common.CodeIdentityAccountInactive, message)
}

// invalidOperatorInput answers a rejected operator profile or invitation
// value. A too weak password names its rule; everything else is an input
// error at the field the validator named.
func invalidOperatorInput(err error, passwordField string) render.Renderer {
	if field, code, ok := operatorInputField(err, passwordField); ok {
		return common.OperatorInvalidField(code, field, err.Error())
	}
	return common.OperatorInvalidRequest(err)
}
