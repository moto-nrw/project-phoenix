package catererauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/inbound/portalauth"
)

// The caterer's own management of its second factor, its passkeys and its
// remembered devices (#4497). The answers have the shapes of the school
// portals' routes, so the portals share their frontend.

var errInvalidPasskeyID = errors.New("invalid passkey id")

// MFAStatusResponse is whether the caterer switched its second factor on.
type MFAStatusResponse struct {
	Enabled     bool   `json:"enabled"`
	MaskedEmail string `json:"masked_email"`
}

// MFAChallengeResponse names the code mailed to switch the factor on.
type MFAChallengeResponse struct {
	ChallengeToken string `json:"challenge_token"`
	MaskedEmail    string `json:"masked_email"`
}

type confirmMFARequest struct {
	ChallengeToken string `json:"challenge_token"`
	Code           string `json:"code"`
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := common.ParseIDParam(r, name)
	return id, err == nil && id > 0
}

func (rs *Resource) mfaStatus(w http.ResponseWriter, r *http.Request) {
	state, err := rs.secondFactor.CatererMFAStatus(r.Context(), currentAccountID(r.Context()))
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	render.JSON(w, r, MFAStatusResponse(state))
}

// startMFA mails the code that switches the second factor on.
func (rs *Resource) startMFA(w http.ResponseWriter, r *http.Request) {
	challenge, err := rs.secondFactor.StartCatererMFA(r.Context(), currentAccountID(r.Context()), common.ParseClientIP(r))
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	render.JSON(w, r, MFAChallengeResponse(challenge))
}

// confirmMFA switches the second factor on and answers with a session
// proven by it; the account's older sessions end at their next refresh.
func (rs *Resource) confirmMFA(w http.ResponseWriter, r *http.Request) {
	var request confirmMFARequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	verify := common.MFAVerifyRequest{ChallengeToken: request.ChallengeToken, Code: request.Code}
	if err := verify.Bind(r); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	session, err := rs.secondFactor.ConfirmCatererMFA(r.Context(), identityaccess.CatererMFAConfirmation{
		AccountID: currentAccountID(r.Context()), ChallengeToken: verify.ChallengeToken, Code: verify.Code,
		IPAddress: common.GetClientIPString(r), UserAgent: r.UserAgent(),
	})
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	render.JSON(w, r, portalauth.TokenResponse{AccessToken: session.AccessToken, RefreshToken: session.RefreshToken})
}

func (rs *Resource) disableMFA(w http.ResponseWriter, r *http.Request) {
	portalauth.SwitchOffFactor(w, r, func(ctx context.Context, currentPassword string, client portalauth.Client) error {
		return rs.secondFactor.DisableCatererMFA(ctx, currentAccountID(ctx), currentPassword, client.IP, client.UserAgent)
	}, renderFailure)
}

func (rs *Resource) listTrustedDevices(w http.ResponseWriter, r *http.Request) {
	portalauth.ListTrustedDevices(w, r, func(ctx context.Context) ([]identityaccess.AccountTrustedDevice, error) {
		return rs.secondFactor.ListCatererTrustedDevices(ctx, currentAccountID(ctx))
	}, renderFailure)
}

func (rs *Resource) revokeTrustedDevice(w http.ResponseWriter, r *http.Request) {
	portalauth.RevokeTrustedDevice(w, r, func(ctx context.Context, deviceID int64) error {
		return rs.secondFactor.RevokeCatererTrustedDevice(ctx, currentAccountID(ctx), deviceID)
	}, renderFailure)
}

func (rs *Resource) listPasskeys(w http.ResponseWriter, r *http.Request) {
	passkeys, err := rs.secondFactor.ListCatererPasskeys(r.Context(), currentAccountID(r.Context()))
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	render.JSON(w, r, passkeys)
}

func (rs *Resource) revokePasskey(w http.ResponseWriter, r *http.Request) {
	passkeyID, ok := pathID(r, "passkeyId")
	if !ok {
		common.RenderError(w, r, common.ErrorInvalidRequest(errInvalidPasskeyID))
		return
	}
	if err := rs.secondFactor.RevokeCatererPasskey(r.Context(), currentAccountID(r.Context()), passkeyID); err != nil {
		renderFailure(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

// passkeyEnrollmentChallenge mails the code a passkey registration needs.
func (rs *Resource) passkeyEnrollmentChallenge(w http.ResponseWriter, r *http.Request) {
	challenge, err := rs.secondFactor.StartCatererPasskeyEnrollment(r.Context(), currentAccountID(r.Context()), common.ParseClientIP(r))
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	render.JSON(w, r, challenge)
}

func (rs *Resource) passkeyRegisterOptions(w http.ResponseWriter, r *http.Request) {
	req := &common.PasskeyRegisterOptionsRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	options, err := rs.secondFactor.BeginCatererPasskeyRegistration(r.Context(), identityaccess.CatererPasskeyRegistrationStart{
		AccountID: currentAccountID(r.Context()), ExpectedOrigin: common.PasskeyOrigin(r), Code: req.Code, Name: req.Name,
	})
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	render.JSON(w, r, options)
}

func (rs *Resource) passkeyRegisterVerify(w http.ResponseWriter, r *http.Request) {
	req := &common.PasskeyRegisterVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	summary, err := rs.secondFactor.FinishCatererPasskeyRegistration(r.Context(), identityaccess.AccountPasskeyRegistrationFinish{
		AccountID: currentAccountID(r.Context()), SessionID: req.SessionID, CredentialResponse: req.Response, Name: req.Name,
	})
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	render.JSON(w, r, summary)
}
