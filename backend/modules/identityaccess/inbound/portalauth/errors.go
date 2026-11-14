// Package portalauth is the sign-in kit every account and operator portal of
// Identity & Access shares: the owner's outcome tables per operation, the
// trusted-device cookie, the second-factor exchange, the login and token
// bodies and the client facts a sign-in records. A portal appends its own
// rules only where it answers differently on purpose.
package portalauth

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
)

// Cause strips the owner's operation envelope, so a table matches and a
// body names the refusal itself.
func Cause(err error) error {
	var operation *identityaccess.AuthenticationError
	if errors.As(err, &operation) && operation.Err != nil {
		return operation.Err
	}
	return err
}

// Renderer classifies err by the tables in order; the first match wins. An
// unclassified error answers 500 and keeps its full text for the log.
func Renderer(err error, tables ...[]common.ErrorRule) render.Renderer {
	return RendererOr(err, common.ErrorInternalServer, tables...)
}

// RendererOr is Renderer with the answer of an unclassified error; it
// receives the full error.
func RendererOr(err error, fallback func(error) render.Renderer, tables ...[]common.ErrorRule) render.Renderer {
	return common.RenderWithRules(Cause(err), slices.Concat(tables...), func(error) render.Renderer {
		return fallback(err)
	})
}

// Render writes the answer Renderer picks.
func Render(w http.ResponseWriter, r *http.Request, err error, tables ...[]common.ErrorRule) {
	common.RenderError(w, r, Renderer(err, tables...))
}

func coded(status func(error, string) render.Renderer, body error, code string) func(error) render.Renderer {
	return common.FixedCodedRenderer(status, body, code)
}

func rule(target error, render func(error) render.Renderer) common.ErrorRule {
	return common.ErrorRule{Target: target, Render: render}
}

// notBooked answers a school that has not booked the portal's module
// (#4895). It is reachable only after the credential was accepted.
var notBooked = common.ErrorRule{
	Match: func(err error) bool {
		_, ok := identityaccess.PortalNotBooked(err)
		return ok
	},
	Render: func(err error) render.Renderer {
		refusal, _ := identityaccess.PortalNotBooked(err)
		return common.ErrorModuleNotBooked(refusal, refusal.Module, refusal.ModuleLabel)
	},
}

var invalidCredentials = coded(common.ErrorUnauthorizedWithCode, identityaccess.ErrInvalidCredentials, common.CodeIdentityInvalidCredentials)

// blocked and unavailable are the second factor's two refusals a sign-in
// shares with every MFA step.
var (
	blocked = []common.ErrorRule{
		rule(identityaccess.ErrMFARateLimited, mfaBlocked),
		rule(identityaccess.ErrMFALocked, mfaBlocked),
	}
	unavailable = rule(identityaccess.ErrMFAStatusUnavailable, coded(common.ErrorServiceUnavailableWithCode, identityaccess.ErrMFAStatusUnavailable, common.CodeIdentityMfaStatusUnavailable))
)

func mfaBlocked(err error) render.Renderer {
	return common.ErrorTooManyRequestsWithCode(err, common.CodeIdentityMfaBlocked)
}

// SessionRules classify why an account may not hold the portal's session:
// the outcomes every sign-in, second-factor exchange and passkey login
// share. An unknown address, a wrong password and a membership at another
// school answer alike, so a sign-in tells nobody which addresses exist;
// everything else is reached only after the credential was accepted.
var SessionRules = []common.ErrorRule{
	notBooked,
	rule(identityaccess.ErrInvalidCredentials, invalidCredentials),
	rule(identityaccess.ErrAccountNotFound, invalidCredentials),
	rule(identityaccess.ErrTenantAccessDenied, coded(common.ErrorUnauthorizedWithCode, identityaccess.ErrTenantAccessDenied, common.CodeIdentityInvalidCredentials)),
	rule(identityaccess.ErrAccountInactive, coded(common.ErrorUnauthorizedWithCode, identityaccess.ErrAccountInactive, common.CodeIdentitySessionAccountInactive)),
	rule(identityaccess.ErrTenantNotFound, common.FixedRenderer(common.ErrorNotFound, identityaccess.ErrTenantNotFound)),
	rule(identityaccess.ErrParentMustUseParentPortal, coded(common.ErrorForbiddenWithCode, identityaccess.ErrParentMustUseParentPortal, common.CodeIdentityUseParentPortal)),
	rule(identityaccess.ErrMustUseSchoolPortal, coded(common.ErrorForbiddenWithCode, identityaccess.ErrMustUseSchoolPortal, common.CodeIdentityUseSchoolPortal)),
	rule(identityaccess.ErrAccountNoSchoolPortalRole, coded(common.ErrorForbiddenWithCode, identityaccess.ErrAccountNoSchoolPortalRole, common.CodeSchoolNoSchoolPortalRole)),
	rule(identityaccess.ErrAccountNoGuardianRole, coded(common.ErrorForbiddenWithCode, identityaccess.ErrAccountNoGuardianRole, common.CodeCareNotAGuardian)),
	rule(identityaccess.ErrNoTraegerMembership, coded(common.ErrorForbiddenWithCode, identityaccess.ErrNoTraegerMembership, common.CodeTraegerNoMembership)),
}

// ParentLoginRules precede LoginRules on the parents portal: its login page
// names a masked refusal by the care code it has always shown.
var ParentLoginRules = []common.ErrorRule{
	rule(identityaccess.ErrInvalidCredentials, coded(common.ErrorUnauthorizedWithCode, identityaccess.ErrInvalidCredentials, common.CodeCareInvalidCredentials)),
	rule(identityaccess.ErrAccountNotFound, coded(common.ErrorUnauthorizedWithCode, identityaccess.ErrInvalidCredentials, common.CodeCareInvalidCredentials)),
}

// WrongPassword reports a refused password check of a signed-in account.
func WrongPassword(err error) bool {
	return errors.Is(err, identityaccess.ErrInvalidCredentials) || errors.Is(err, identityaccess.ErrAccountNotFound)
}

// LoginRules classify a refused password sign-in.
var LoginRules = slices.Concat(SessionRules, blocked, []common.ErrorRule{unavailable})

// ExchangeRules classify a refused mint after a proven second factor. A
// proof an MFA reset revoked between the code and the mint answers like a
// wrong code.
var ExchangeRules = slices.Concat([]common.ErrorRule{
	rule(identityaccess.ErrInvalidToken, coded(common.ErrorUnauthorizedWithCode, identityaccess.ErrMFACodeInvalid, common.CodeIdentityMfaCodeInvalid)),
}, SessionRules)

// MFARules classify the second-factor steps: verify, resend, enrollment and
// the trusted devices. A wrong code, an unusable challenge and a challenge
// of another portal answer with one code, so the step tells nobody which it
// was.
var MFARules = slices.Concat([]common.ErrorRule{
	rule(identityaccess.ErrMFAChallengeTokenInvalid, mfaCodeInvalid),
	rule(identityaccess.ErrMFACodeInvalid, mfaCodeInvalid),
	rule(identityaccess.ErrMFAUnsupportedScope, mfaCodeInvalid),
	// A resend or an enrollment start bound to a password a change has
	// replaced since (#6289): the step asks for a new sign-in, as the
	// sign-in with the old password itself is refused.
	rule(identityaccess.ErrInvalidCredentials, invalidCredentials),
}, blocked, []common.ErrorRule{
	unavailable,
	rule(identityaccess.ErrMFANotEnrolled, common.ErrorForbidden),
	rule(identityaccess.ErrMFAAlreadyEnrolled, common.ErrorConflict),
	rule(identityaccess.ErrMFAPermissionDenied, common.ErrorForbidden),
	// A school administrator's account-wide reset of an account another
	// school or a Träger also holds (#6862).
	rule(identityaccess.ErrAccountSharedAcrossSchools, AccountSharedAcrossSchools),
	rule(identityaccess.ErrMFAInvalidOverride, common.ErrorInvalidRequest),
})

// AccountSharedAcrossSchools answers a school administrator's change to
// what holds for a whole account while another school or a Träger also
// holds it (#6862).
func AccountSharedAcrossSchools(err error) render.Renderer {
	return common.ErrorForbiddenWithCode(err, common.CodeIdentityAccountSharedAcrossSchools)
}

func mfaCodeInvalid(err error) render.Renderer {
	return common.ErrorUnauthorizedWithCode(err, common.CodeIdentityMfaCodeInvalid)
}

// SwitchRules classify a refused switch of a live session to another school.
// The source session must prove a live second factor; its loss asks for a
// new sign-in.
var SwitchRules = []common.ErrorRule{
	notBooked,
	rule(identityaccess.ErrInvalidToken, coded(common.ErrorUnauthorizedWithCode, errMFASessionRequired, common.CodeIdentityMfaSessionRequired)),
	unavailable,
	rule(identityaccess.ErrAccountNotFound, coded(common.ErrorUnauthorizedWithCode, identityaccess.ErrAccountNotFound, common.CodeIdentityAccountNotFound)),
	rule(identityaccess.ErrAccountInactive, coded(common.ErrorUnauthorizedWithCode, identityaccess.ErrAccountInactive, common.CodeIdentitySessionAccountInactive)),
	rule(identityaccess.ErrTenantNotFound, common.FixedRenderer(common.ErrorNotFound, identityaccess.ErrTenantNotFound)),
	rule(identityaccess.ErrTenantAccessDenied, coded(common.ErrorUnauthorizedWithCode, identityaccess.ErrTenantAccessDenied, common.CodeIdentityTenantAccessDenied)),
	rule(identityaccess.ErrMustUseSchoolPortal, coded(common.ErrorForbiddenWithCode, identityaccess.ErrMustUseSchoolPortal, common.CodeIdentityUseSchoolPortal)),
	rule(identityaccess.ErrAccountNoSchoolPortalRole, coded(common.ErrorForbiddenWithCode, identityaccess.ErrAccountNoSchoolPortalRole, common.CodeSchoolNoSchoolPortalRole)),
	rule(identityaccess.ErrDemoSessionTenantLocked, coded(common.ErrorForbiddenWithCode, identityaccess.ErrDemoSessionTenantLocked, common.CodeIdentityDemoSession)),
}

var errMFASessionRequired = errors.New("switching requires a live MFA-verified session; sign in again")

// RenewalRules classify a refused refresh or session validation by
// identityaccess.SessionOutcomeOf, so both routes answer one state alike: an
// ended session a plain 401, which the frontend turns into a clean logout,
// a missing module 403 module.not_booked with the presented token untouched.
var RenewalRules = []common.ErrorRule{
	notBooked,
	unavailable,
	{
		Match:  func(err error) bool { return identityaccess.SessionOutcomeOf(err) == identityaccess.SessionEnded },
		Render: common.ErrorUnauthorized,
	},
}

// PasskeyLoginRules answer every refused passkey sign-in with one code, so
// the login page says the same for an unknown passkey, a foreign ceremony
// and an account of another portal.
var PasskeyLoginRules = []common.ErrorRule{
	rule(identityaccess.ErrInvalidCredentials, passkeyLoginFailed(identityaccess.ErrInvalidCredentials)),
	rule(identityaccess.ErrPasskeySessionInvalid, passkeyLoginFailed(identityaccess.ErrInvalidCredentials)),
	rule(identityaccess.ErrAccountNotFound, passkeyLoginFailed(identityaccess.ErrInvalidCredentials)),
	rule(identityaccess.ErrTenantAccessDenied, passkeyLoginFailed(identityaccess.ErrTenantAccessDenied)),
	rule(identityaccess.ErrPasskeyOriginInvalid, passkeyLoginFailed(identityaccess.ErrPasskeyOriginInvalid)),
}

func passkeyLoginFailed(body error) func(error) render.Renderer {
	return coded(common.ErrorUnauthorizedWithCode, body, common.CodeIdentityPasskeyLoginFailed)
}

// PasskeyRules classify the passkey ceremonies: options, registration,
// listing and revocation. Registering starts an e-mail challenge, so the
// second factor's refusals answer as in the MFA steps.
var PasskeyRules = slices.Concat([]common.ErrorRule{
	rule(identityaccess.ErrInvalidCredentials, common.FixedRenderer(common.ErrorUnauthorized, identityaccess.ErrInvalidCredentials)),
	rule(identityaccess.ErrPasskeySessionInvalid, common.FixedRenderer(common.ErrorUnauthorized, identityaccess.ErrInvalidCredentials)),
	rule(identityaccess.ErrAccountNotFound, common.FixedRenderer(common.ErrorUnauthorized, identityaccess.ErrInvalidCredentials)),
	rule(identityaccess.ErrAccountInactive, coded(common.ErrorUnauthorizedWithCode, identityaccess.ErrAccountInactive, common.CodeIdentitySessionAccountInactive)),
	rule(identityaccess.ErrTenantAccessDenied, common.FixedRenderer(common.ErrorUnauthorized, identityaccess.ErrTenantAccessDenied)),
	rule(identityaccess.ErrTenantNotFound, common.FixedRenderer(common.ErrorNotFound, identityaccess.ErrTenantNotFound)),
	rule(identityaccess.ErrPasskeyOriginInvalid, common.ErrorUnauthorized),
	rule(identityaccess.ErrMFACodeInvalid, mfaCodeInvalid),
}, blocked, []common.ErrorRule{
	unavailable,
	rule(identityaccess.ErrParentMustUseParentPortal, coded(common.ErrorForbiddenWithCode, identityaccess.ErrParentMustUseParentPortal, common.CodeIdentityUseParentPortal)),
	rule(identityaccess.ErrMustUseSchoolPortal, coded(common.ErrorForbiddenWithCode, identityaccess.ErrMustUseSchoolPortal, common.CodeIdentityUseSchoolPortal)),
	rule(identityaccess.ErrPasskeyNotFound, common.ErrorNotFound),
})

// PasskeyFailed is the 500 of a passkey record that could not be read or
// written; the cause goes to the log only.
func PasskeyFailed(err error) render.Renderer {
	return common.ErrorInternalServerWrap("passkey request failed", err)
}

// PasswordResetLinkUnusable reports a link that is unknown, expired or
// already spent. A reset never says which of the three it was.
func PasswordResetLinkUnusable(err error) bool {
	cause := Cause(err)
	return errors.Is(cause, identityaccess.ErrInvalidToken) || errors.Is(cause, identityaccess.ErrRecordMissing)
}

// PasswordTooWeak reports a credential the password policy refused.
func PasswordTooWeak(err error) bool {
	return errors.Is(Cause(err), identityaccess.ErrPasswordTooWeak)
}

// PasswordResetRetryAfter reports the rate limit and the seconds until the
// address may request again.
func PasswordResetRetryAfter(err error) (int, bool) {
	var limited *identityaccess.PasswordResetRateLimitError
	if !errors.As(err, &limited) {
		return 0, false
	}
	return limited.RetryAfterSeconds(time.Now()), true
}

// PasswordResetRoutes are the public reset routes of one portal: only that
// portal's accounts are served, and the link lands on its host.
func PasswordResetRoutes(resets identityaccess.PasswordResets, scope identityaccess.PasswordResetScope) common.PasswordResetRoutes {
	if resets == nil {
		return common.PasswordResetRoutes{}
	}
	return common.PasswordResetRoutes{
		Initiate: func(ctx context.Context, email string) error {
			_, err := resets.InitiatePasswordReset(ctx, email, scope)
			return err
		},
		Reset:        resets.ResetPassword,
		LinkUnusable: PasswordResetLinkUnusable,
		TooWeak:      PasswordTooWeak,
		RetryAfter:   PasswordResetRetryAfter,
	}
}

// ParentSecondFactorRules classify the parents portal's own management of
// its second factor (#7051), ahead of the shared code and session tables.
// Switching off a factor the account does not have and switching it on
// without a school of the account are business refusals; the settings show
// neither action in those states.
var ParentSecondFactorRules = slices.Concat([]common.ErrorRule{
	rule(identityaccess.ErrParentMFANotEnabled, common.FixedRenderer(common.ErrorConflict, identityaccess.ErrParentMFANotEnabled)),
	rule(identityaccess.ErrParentMFANeedsSchool, common.FixedRenderer(common.ErrorConflict, identityaccess.ErrParentMFANeedsSchool)),
	rule(identityaccess.ErrAccountMFAUnavailable, common.FixedRenderer(common.ErrorServiceUnavailable, identityaccess.ErrAccountMFAUnavailable)),
}, ExchangeRules, MFARules)
