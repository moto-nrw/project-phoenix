package traegerportal

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/inbound/portalauth"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
)

// The code step in the running Träger session (#7068). A school without
// remembered devices refuses an Absprung whose factor only a remembered
// device proved (403 traeger.school_requires_code). The person then asks for
// a code here and redeems it without signing out: Identity & Access mails an
// org-scope code with the sign-in's limits and, once it is redeemed, rotates
// the Träger session into a family the code proved. The answer is the new
// session pair, which the BFF installs in place of the old one.

var errStepUpUnavailable = errors.New("the Träger code step is not composed")

// StepUpStartResponse is the answer of POST /traeger/auth/mfa/step-up.
type StepUpStartResponse struct {
	ChallengeToken string `json:"challenge_token"`
	MaskedEmail    string `json:"masked_email"`
}

// stepUpStart mails the code for the account of the Träger session.
func (rs *Resource) stepUpStart(w http.ResponseWriter, r *http.Request) {
	request, ok := rs.stepUpRequest(w, r)
	if !ok {
		return
	}
	challenge, err := rs.stepUp.StartTraegerStepUp(r.Context(), request)
	if err != nil {
		portalauth.Render(w, r, err, portalauth.MFARules, portalauth.ExchangeRules)
		return
	}
	render.JSON(w, r, StepUpStartResponse{ChallengeToken: challenge.ChallengeToken, MaskedEmail: challenge.MaskedEmail})
}

// stepUpVerify redeems the code of the session's account and answers with
// the rotated session pair. The challenge is pinned to the account before
// the code is compared, so a code of another account is never consumed.
func (rs *Resource) stepUpVerify(w http.ResponseWriter, r *http.Request) {
	request, ok := rs.stepUpRequest(w, r)
	if !ok {
		return
	}
	if !rs.requireMFA(w, r) {
		return
	}
	req := &common.MFAVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	verified, err := rs.mfa.VerifyChallengeForOwner(r.Context(), req.ChallengeToken, req.Code, request.AccountID)
	if err != nil {
		portalauth.Render(w, r, err, portalauth.MFARules)
		return
	}
	result, err := rs.stepUp.CompleteTraegerStepUp(r.Context(), request, identityaccess.VerifiedMFAChallenge{
		AccountID: verified.AccountID, Scope: identityaccess.MFAChallengeScopeOrg, MFAResetGeneration: verified.MFAResetGeneration,
	})
	if err != nil {
		portalauth.Render(w, r, err, portalauth.ExchangeRules, portalauth.LoginRules)
		return
	}
	render.JSON(w, r, portalauth.TokenResponse{AccessToken: result.AccessToken, RefreshToken: result.RefreshToken})
}

// stepUpRequest names the Träger session of the request: account, Träger
// and family come from its token, never from the body.
func (rs *Resource) stepUpRequest(w http.ResponseWriter, r *http.Request) (identityaccess.TraegerStepUpRequest, bool) {
	if !common.RequireDependency(w, r, rs.stepUp != nil, errStepUpUnavailable) {
		return identityaccess.TraegerStepUpRequest{}, false
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID <= 0 || claims.OrgID <= 0 || claims.FamilyID == "" || !claims.IsOrgScope() {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return identityaccess.TraegerStepUpRequest{}, false
	}
	return identityaccess.TraegerStepUpRequest{
		AccountID: int64(claims.ID), OrganizationID: claims.OrgID, FamilyID: claims.FamilyID,
		IPAddress: common.GetClientIPString(r), UserAgent: r.UserAgent(),
	}, true
}
