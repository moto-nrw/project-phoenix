package parentauth

import (
	"net/http"
	"slices"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/inbound/portalauth"
)

// loginRules precede the shared sign-in table: the parents portal's login
// page names a masked refusal by the care code it has always shown.
var loginRules = []common.ErrorRule{
	{Target: identityaccess.ErrInvalidCredentials, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, identityaccess.ErrInvalidCredentials, common.CodeCareInvalidCredentials)},
	{Target: identityaccess.ErrAccountNotFound, Render: common.FixedCodedRenderer(common.ErrorUnauthorizedWithCode, identityaccess.ErrInvalidCredentials, common.CodeCareInvalidCredentials)},
}

// signInRules classify a refused password sign-in and a refused code: the
// parents portal's masked refusals first, then the mint after a proven code,
// the code steps and the password sign-in.
var signInRules = slices.Concat(loginRules, portalauth.ExchangeRules, portalauth.MFARules, portalauth.LoginRules)

// secondFactorRules classify the parent's own management of the second
// factor and its remembered devices (#7051), ahead of the shared code and
// session tables. Switching off a factor the account does not have and
// switching it on without a school of the account are business refusals;
// the settings show neither action in those states.
var secondFactorRules = slices.Concat([]common.ErrorRule{
	{Target: identityaccess.ErrParentMFANotEnabled, Render: common.FixedRenderer(common.ErrorConflict, identityaccess.ErrParentMFANotEnabled)},
	{Target: identityaccess.ErrParentMFANeedsSchool, Render: common.FixedRenderer(common.ErrorConflict, identityaccess.ErrParentMFANeedsSchool)},
	{Target: identityaccess.ErrAccountMFAUnavailable, Render: common.FixedRenderer(common.ErrorServiceUnavailable, identityaccess.ErrAccountMFAUnavailable)},
}, portalauth.ExchangeRules, portalauth.MFARules)

func renderFailure(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, portalauth.RendererOr(err, common.ErrorInternalServerRenderer("parent second factor failed"), secondFactorRules))
}
