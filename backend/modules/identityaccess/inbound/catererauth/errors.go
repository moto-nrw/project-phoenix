package catererauth

import (
	"net/http"
	"slices"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/portalauth"
)

// catererRefusals are the refusals only the caterer portal has. They come
// first; everything else answers like on every account portal.
var catererRefusals = []common.ErrorRule{
	// An account that is no caterer's answers like a wrong password, so the
	// caterer login tells nobody which addresses hold other accounts.
	{Target: identityaccess.ErrAccountNotCaterer, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, identityaccess.ErrInvalidCredentials, common.CodeIdentityInvalidCredentials)},
	{Target: identityaccess.ErrCatererWithoutSchool, Render: common.FixedCodedRenderer(common.ErrorForbiddenWithCode, identityaccess.ErrCatererWithoutSchool, common.CodeIdentityCatererWithoutSchool)},
	{Target: identityaccess.ErrCatererMFANotEnabled, Render: common.FixedRenderer(common.ErrorConflict, identityaccess.ErrCatererMFANotEnabled)},
	{Target: identityaccess.ErrAccountMFAUnavailable, Render: common.FixedRenderer(common.ErrorServiceUnavailable, identityaccess.ErrAccountMFAUnavailable)},
	{Target: identityaccess.ErrPasskeyFlowsUnavailable, Render: common.FixedRenderer(common.ErrorServiceUnavailable, identityaccess.ErrPasskeyFlowsUnavailable)},
}

// rules classify a refused caterer sign-in, its second-factor code and the
// caterer's management of its factor, passkeys and remembered devices: the
// session refusals and the mint after a proven code first, so a wrong
// password answers alike everywhere, then the code and passkey steps.
var rules = slices.Concat(catererRefusals, portalauth.ExchangeRules, portalauth.MFARules, portalauth.PasskeyRules)

// passkeyLoginRules answer every refused passkey sign-in with one code, an
// account that is no caterer's included.
var passkeyLoginRules = slices.Concat([]common.ErrorRule{{
	Target: identityaccess.ErrAccountNotCaterer,
	Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, identityaccess.ErrInvalidCredentials, common.CodeIdentityPasskeyLoginFailed),
}}, portalauth.PasskeyLoginRules, rules)

// renderRefusal answers err by table. An unclassified error answers 500
// with a fixed text and keeps its cause for the log only.
func renderRefusal(w http.ResponseWriter, r *http.Request, err error, table []common.ErrorRule, failed string) {
	common.RenderError(w, r, portalauth.RendererOr(err, common.ErrorInternalServerRenderer(failed), table))
}

func renderFailure(w http.ResponseWriter, r *http.Request, err error) {
	renderRefusal(w, r, err, rules, "caterer second factor failed")
}

func renderPasskeyLoginFailure(w http.ResponseWriter, r *http.Request, err error) {
	renderRefusal(w, r, err, passkeyLoginRules, "caterer passkey sign-in failed")
}
