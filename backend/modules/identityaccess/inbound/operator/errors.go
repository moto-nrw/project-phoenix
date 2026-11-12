package operator

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
)

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
		return ProfileErrorRenderer(err)
	}
}

// accessError surfaces validation messages verbatim: the operator needs to
// know why a role was rejected.
func (rs *Resource) accessError(err error) render.Renderer {
	if invalid, ok := errors.AsType[*identityaccess.InvalidInputError](err); ok {
		if errors.Is(invalid.Err, identityaccess.ErrLehrkraftRoleImmutable) {
			return common.OperatorRejection(http.StatusBadRequest, common.CodeIdentityLehrkraftRoleImmutable, invalid.Err.Error())
		}
		return common.OperatorInvalidRequest(invalid.Err)
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
	case errors.Is(err, identityaccess.ErrEmailAlreadyExists):
		// A caterer account joins no school (ADR 0077).
		return common.OperatorRejection(http.StatusConflict, common.CodeIdentityEmailAlreadyExists, "account cannot join a school")
	default:
		return rs.accessFallback(err)
	}
}

// authRules classify the operator sign-in, refresh and second factor. An
// unknown operator answers like a wrong password.
var authRules = []common.ErrorRule{
	{Target: identityaccess.ErrOperatorRefreshTokenInvalid, Render: func(error) render.Renderer { return common.OperatorUnauthorized() }},
	{Target: identityaccess.ErrOperatorInvalidCredentials, Render: func(error) render.Renderer { return common.OperatorInvalidCredentials() }},
	{Target: identityaccess.ErrOperatorNotFound, Render: func(error) render.Renderer { return common.OperatorInvalidCredentials() }},
	{Target: identityaccess.ErrOperatorInactive, Render: func(error) render.Renderer { return operatorInactive("Operator account is inactive") }},
	{Target: identityaccess.ErrMFARateLimited, Render: operatorRejection(http.StatusTooManyRequests, common.CodeIdentityMfaBlocked, "Too many code requests, please wait")},
	{Target: identityaccess.ErrMFALocked, Render: operatorRejection(http.StatusTooManyRequests, common.CodeIdentityMfaBlocked, "MFA account temporarily locked")},
	{Target: identityaccess.ErrMFAChallengeTokenInvalid, Render: operatorRejection(http.StatusUnauthorized, common.CodeIdentityMfaCodeInvalid, "Invalid email or password")},
	{Target: identityaccess.ErrMFACodeInvalid, Render: operatorRejection(http.StatusUnauthorized, common.CodeIdentityMfaCodeInvalid, "Invalid email or password")},
	{Target: identityaccess.ErrMFAStatusUnavailable, Render: func(error) render.Renderer {
		return common.OperatorServiceUnavailable("MFA status temporarily unavailable, please retry")
	}},
}

// AuthErrorRenderer answers an operator sign-in outcome; everything the
// table does not name is a failed sign-in.
var AuthErrorRenderer = common.RulesRenderer(authRules, func(error) render.Renderer {
	return common.OperatorInternal("Authentication failed")
})

func operatorRejection(status int, code, message string) func(error) render.Renderer {
	return func(error) render.Renderer { return common.OperatorRejection(status, code, message) }
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
