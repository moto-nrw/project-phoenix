package catererportalhttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/workflows/catererportal"
)

var (
	errInvalidCredentials = errors.New("invalid email or password")
	errAccountInactive    = errors.New("account is inactive")
)

// errorRules map the caterer flows' errors to their codes. A sign-in that
// fails for any reason of the account answers like a wrong password, so the
// caterer login tells nobody which addresses hold other accounts.
var errorRules = []common.ErrorRule{
	{Target: identityaccess.ErrCatererAlreadySupplies, Render: common.FixedCodedRenderer(common.ErrorConflictWithCode, identityaccess.ErrCatererAlreadySupplies, common.CodeMealsSupplyActive)},
	{Target: catererportal.ErrDeliveryRangeTooLong, Render: common.FixedCodedRenderer(common.ErrorInvalidRequestWithCode, catererportal.ErrDeliveryRangeTooLong, common.CodeMealsDeliveryRangeTooLong)},
	{Target: catererportal.ErrSchoolNotSupplied, Render: common.FixedCodedRenderer(common.ErrorForbiddenWithCode, catererportal.ErrSchoolNotSupplied, common.CodeMealsNotSupplied)},
	{Target: mealplan.ErrOrderNotFound, Render: common.FixedCodedRenderer(common.ErrorNotFoundWithCode, errors.New(mealplan.ErrOrderNotFound.Detail), common.CodeMealsOrderNotFound)},
	{Target: mealplan.ErrDisabled, Render: common.FixedCodedRenderer(common.ErrorForbiddenWithCode, errors.New(mealplan.ErrDisabled.Detail), common.CodeMealsPlanDisabled)},
	{Target: mealplan.ErrRegistrationDisabled, Render: common.FixedCodedRenderer(common.ErrorForbiddenWithCode, errors.New(mealplan.ErrRegistrationDisabled.Detail), common.CodeMealsRegistrationDisabled)},
	{Target: mealplan.ErrInvalidParticipation, Render: common.FixedRenderer(common.ErrorInvalidRequest, errors.New("the window must be at most six weeks"))},
	{Target: identityaccess.ErrCatererAddressTaken, Render: common.FixedCodedRenderer(common.ErrorConflictWithCode, identityaccess.ErrCatererAddressTaken, common.CodeIdentityCatererAddressTaken)},
	{Target: identityaccess.ErrCatererWithoutSchool, Render: common.FixedCodedRenderer(common.ErrorForbiddenWithCode, identityaccess.ErrCatererWithoutSchool, common.CodeIdentityCatererWithoutSchool)},
	{Target: identityaccess.ErrInvalidCatererInvitation, Render: common.FixedRenderer(common.ErrorInvalidRequest, identityaccess.ErrInvalidCatererInvitation)},
	{Target: identityaccess.ErrInvitationNotFound, Render: common.FixedCodedRenderer(common.ErrorNotFoundWithCode, identityaccess.ErrInvitationNotFound, common.CodeIdentityInvitationNotFound)},
	{Target: identityaccess.ErrInvitationExpired, Render: common.FixedCodedRenderer(common.ErrorGoneWithCode, identityaccess.ErrInvitationExpired, common.CodeIdentityInvitationExpired)},
	{Target: identityaccess.ErrInvitationUsed, Render: common.FixedCodedRenderer(common.ErrorGoneWithCode, identityaccess.ErrInvitationUsed, common.CodeIdentityInvitationExpired)},
	{Target: identityaccess.ErrInvitationRevoked, Render: common.FixedCodedRenderer(common.ErrorGoneWithCode, identityaccess.ErrInvitationRevoked, common.CodeIdentityInvitationExpired)},
	{Target: identityaccess.ErrInvitationTenantDeleted, Render: common.FixedCodedRenderer(common.ErrorNotFoundWithCode, identityaccess.ErrInvitationNotFound, common.CodeIdentityInvitationNotFound)},
	{Target: identityaccess.ErrInvitationPasswordMismatch, Render: func(err error) render.Renderer {
		return common.ErrorInvalidOnField(identityaccess.ErrInvitationPasswordMismatch, common.CodeIdentityPasswordMismatch, "confirm_password")
	}},
	{Target: identityaccess.ErrPasswordTooWeak, Render: func(err error) render.Renderer {
		return common.ErrorInvalidOnField(identityaccess.ErrPasswordTooWeak, common.CodeIdentityPasswordTooWeak, "password")
	}},
	{Target: identityaccess.ErrAccountInactive, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, errAccountInactive, common.CodeIdentitySessionAccountInactive)},
	{Target: identityaccess.ErrInvalidCredentials, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, errInvalidCredentials, common.CodeIdentityInvalidCredentials)},
	{Target: identityaccess.ErrAccountNotFound, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, errInvalidCredentials, common.CodeIdentityInvalidCredentials)},
	{Target: identityaccess.ErrAccountNotCaterer, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, errInvalidCredentials, common.CodeIdentityInvalidCredentials)},
}

// secondFactorRules map the second factor of the caterer (#4497) like the
// school portals' MFA and passkey routes: one code for a wrong code and an
// unusable challenge, a lock and a send cap as blocked, an unreadable status
// as unavailable.
var secondFactorRules = []common.ErrorRule{
	{Target: identityaccess.ErrMFAChallengeTokenInvalid, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, identityaccess.ErrMFACodeInvalid, common.CodeIdentityMfaCodeInvalid)},
	{Target: identityaccess.ErrMFACodeInvalid, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, identityaccess.ErrMFACodeInvalid, common.CodeIdentityMfaCodeInvalid)},
	{Target: identityaccess.ErrMFAUnsupportedScope, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, identityaccess.ErrMFACodeInvalid, common.CodeIdentityMfaCodeInvalid)},
	// A proof an operator's reset revoked between the code and the mint.
	{Target: identityaccess.ErrInvalidToken, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, identityaccess.ErrMFACodeInvalid, common.CodeIdentityMfaCodeInvalid)},
	{Target: identityaccess.ErrMFALocked, Render: common.FixedCodedRenderer(common.ErrorTooManyRequestsWithCode, identityaccess.ErrMFALocked, common.CodeIdentityMfaBlocked)},
	{Target: identityaccess.ErrMFARateLimited, Render: common.FixedCodedRenderer(common.ErrorTooManyRequestsWithCode, identityaccess.ErrMFARateLimited, common.CodeIdentityMfaBlocked)},
	{Target: identityaccess.ErrMFAStatusUnavailable, Render: common.FixedRenderer(common.ErrorServiceUnavailable, identityaccess.ErrMFAStatusUnavailable)},
	{Target: identityaccess.ErrAccountMFAUnavailable, Render: common.FixedRenderer(common.ErrorServiceUnavailable, identityaccess.ErrAccountMFAUnavailable)},
	{Target: identityaccess.ErrPasskeyFlowsUnavailable, Render: common.FixedRenderer(common.ErrorServiceUnavailable, identityaccess.ErrPasskeyFlowsUnavailable)},
	{Target: identityaccess.ErrMFAAlreadyEnrolled, Render: common.FixedRenderer(common.ErrorConflict, identityaccess.ErrMFAAlreadyEnrolled)},
	{Target: identityaccess.ErrCatererMFANotEnabled, Render: common.FixedRenderer(common.ErrorConflict, identityaccess.ErrCatererMFANotEnabled)},
	{Target: identityaccess.ErrMFAPermissionDenied, Render: common.FixedRenderer(common.ErrorForbidden, identityaccess.ErrMFAPermissionDenied)},
	{Target: identityaccess.ErrPasskeyNotFound, Render: common.FixedRenderer(common.ErrorNotFound, identityaccess.ErrPasskeyNotFound)},
	{Target: identityaccess.ErrPasskeySessionInvalid, Render: common.FixedRenderer(common.ErrorUnauthorized, identityaccess.ErrInvalidCredentials)},
	{Target: identityaccess.ErrPasskeyOriginInvalid, Render: common.FixedRenderer(common.ErrorUnauthorized, identityaccess.ErrPasskeyOriginInvalid)},
}

// passkeyLoginRules answer every refused passkey sign-in with one code, so
// the login page says the same for an unknown passkey, a foreign ceremony
// and an account that is no caterer's.
var passkeyLoginRules = []common.ErrorRule{
	{Target: identityaccess.ErrInvalidCredentials, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, identityaccess.ErrInvalidCredentials, common.CodeIdentityPasskeyLoginFailed)},
	{Target: identityaccess.ErrPasskeySessionInvalid, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, identityaccess.ErrInvalidCredentials, common.CodeIdentityPasskeyLoginFailed)},
	{Target: identityaccess.ErrAccountNotFound, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, identityaccess.ErrInvalidCredentials, common.CodeIdentityPasskeyLoginFailed)},
	{Target: identityaccess.ErrAccountNotCaterer, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, identityaccess.ErrInvalidCredentials, common.CodeIdentityPasskeyLoginFailed)},
	{Target: identityaccess.ErrPasskeyOriginInvalid, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, identityaccess.ErrPasskeyOriginInvalid, common.CodeIdentityPasskeyLoginFailed)},
}

func renderSecondFactorFailure(w http.ResponseWriter, r *http.Request, err error) {
	rules := append(append([]common.ErrorRule{}, secondFactorRules...), errorRules...)
	common.RenderError(w, r, common.RenderWithRules(err, rules, common.ErrorInternalServerRenderer("caterer second factor failed")))
}

func renderPasskeyLoginFailure(w http.ResponseWriter, r *http.Request, err error) {
	rules := append(append(append([]common.ErrorRule{}, passkeyLoginRules...), secondFactorRules...), errorRules...)
	common.RenderError(w, r, common.RenderWithRules(err, rules, common.ErrorInternalServerRenderer("caterer passkey sign-in failed")))
}
