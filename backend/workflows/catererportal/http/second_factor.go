package catererportalhttp

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The caterer's second factor (#4497): the code of the sign-in, the passkey
// sign-in, and the caterer's own management of the factor, its passkeys and
// its remembered devices. The answers have the shapes of the school
// portals' routes, so the portals share their frontend.

var (
	errInvalidDeviceID  = errors.New("invalid device id")
	errInvalidPasskeyID = errors.New("invalid passkey id")
	errPasswordRequired = errors.New("current_password is required")
)

// verifyMFA redeems the mailed code of a sign-in.
func (rs *Resource) verifyMFA(w http.ResponseWriter, r *http.Request) {
	req := &common.MFAVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	session, err := rs.runtime.SignIn.VerifyCatererMFA(r.Context(), identityaccess.CatererMFAVerification{
		ChallengeToken: req.ChallengeToken, Code: req.Code, RememberDevice: req.RememberDevice,
		IPAddress: common.GetClientIPString(r), UserAgent: r.UserAgent(),
	})
	if err != nil {
		renderSecondFactorFailure(w, r, err)
		return
	}
	common.SetTrustedDevice(w, session.TrustedDeviceCookie, session.TrustedDeviceExpiresAt)
	render.JSON(w, r, TokenResponse{AccessToken: session.AccessToken, RefreshToken: session.RefreshToken})
}

// resendMFA mails a new code for a caterer challenge.
func (rs *Resource) resendMFA(w http.ResponseWriter, r *http.Request) {
	req := &common.MFAResendRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	renewed, err := rs.runtime.SignIn.ResendCatererMFA(r.Context(), req.ChallengeToken, common.ParseClientIP(r))
	if err != nil {
		renderSecondFactorFailure(w, r, err)
		return
	}
	render.JSON(w, r, common.MFAResendResponse{ChallengeToken: renewed})
}

// passkeyLoginOptions starts a passkey sign-in on the caterer portal.
func (rs *Resource) passkeyLoginOptions(w http.ResponseWriter, r *http.Request) {
	options, err := rs.runtime.SignIn.BeginCatererPasskeyLogin(r.Context(), common.PasskeyOrigin(r))
	if err != nil {
		renderPasskeyLoginFailure(w, r, err)
		return
	}
	render.JSON(w, r, options)
}

// passkeyLoginVerify completes the passkey sign-in.
func (rs *Resource) passkeyLoginVerify(w http.ResponseWriter, r *http.Request) {
	req := &common.PasskeyVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	result, err := rs.runtime.SignIn.FinishCatererPasskeyLogin(r.Context(), identityaccess.AccountPasskeyLoginFinish{
		SessionID: req.SessionID, CredentialResponse: req.Response,
		IPAddress: common.GetClientIPString(r), UserAgent: r.UserAgent(),
	})
	if err != nil {
		renderPasskeyLoginFailure(w, r, err)
		return
	}
	render.JSON(w, r, LoginResponse{
		Status: string(identityaccess.CatererLoginAuthenticated), AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
	})
}

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

type disableMFARequest struct {
	CurrentPassword string `json:"current_password"`
}

func (rs *Resource) mfaStatus(w http.ResponseWriter, r *http.Request) {
	state, err := rs.runtime.SecondFactor.CatererMFAStatus(r.Context(), rs.currentAccountID(r.Context()))
	if err != nil {
		renderSecondFactorFailure(w, r, err)
		return
	}
	render.JSON(w, r, MFAStatusResponse(state))
}

// startMFA mails the code that switches the second factor on.
func (rs *Resource) startMFA(w http.ResponseWriter, r *http.Request) {
	challenge, err := rs.runtime.SecondFactor.StartCatererMFA(r.Context(), rs.currentAccountID(r.Context()), common.ParseClientIP(r))
	if err != nil {
		renderSecondFactorFailure(w, r, err)
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
	session, err := rs.runtime.SecondFactor.ConfirmCatererMFA(r.Context(), identityaccess.CatererMFAConfirmation{
		AccountID: rs.currentAccountID(r.Context()), ChallengeToken: verify.ChallengeToken, Code: verify.Code,
		IPAddress: common.GetClientIPString(r), UserAgent: r.UserAgent(),
	})
	if err != nil {
		renderSecondFactorFailure(w, r, err)
		return
	}
	render.JSON(w, r, TokenResponse{AccessToken: session.AccessToken, RefreshToken: session.RefreshToken})
}

// disableMFA switches the second factor off. A wrong current password
// answers on its field, like the password change.
func (rs *Resource) disableMFA(w http.ResponseWriter, r *http.Request) {
	var request disableMFARequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.CurrentPassword == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errPasswordRequired))
		return
	}
	err := rs.runtime.SecondFactor.DisableCatererMFA(r.Context(), rs.currentAccountID(r.Context()), request.CurrentPassword, common.GetClientIPString(r), r.UserAgent())
	switch {
	case err == nil:
		common.RespondNoContent(w, r)
	case errors.Is(err, identityaccess.ErrInvalidCredentials):
		common.RenderError(w, r, common.ErrorUnauthorizedOnField(errCurrentPasswordWrong, common.CodeIdentityCurrentPasswordWrong, "current_password"))
	default:
		renderSecondFactorFailure(w, r, err)
	}
}

func (rs *Resource) listTrustedDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := rs.runtime.SecondFactor.ListCatererTrustedDevices(r.Context(), rs.currentAccountID(r.Context()))
	if err != nil {
		renderSecondFactorFailure(w, r, err)
		return
	}
	render.JSON(w, r, common.MapTrustedDevices(devices, func(d identityaccess.AccountTrustedDevice) common.TrustedDeviceRow {
		return common.TrustedDeviceRow{
			ID: d.ID, UserAgent: d.UserAgent, IPAddress: d.IPAddress,
			CreatedAt: d.CreatedAt, ExpiresAt: d.ExpiresAt, LastUsedAt: d.LastUsedAt,
		}
	}))
}

func (rs *Resource) revokeTrustedDevice(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := pathID(r, "deviceId")
	if !ok {
		common.RenderError(w, r, common.ErrorInvalidRequest(errInvalidDeviceID))
		return
	}
	if err := rs.runtime.SecondFactor.RevokeCatererTrustedDevice(r.Context(), rs.currentAccountID(r.Context()), deviceID); err != nil {
		renderSecondFactorFailure(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

func (rs *Resource) listPasskeys(w http.ResponseWriter, r *http.Request) {
	passkeys, err := rs.runtime.SecondFactor.ListCatererPasskeys(r.Context(), rs.currentAccountID(r.Context()))
	if err != nil {
		renderSecondFactorFailure(w, r, err)
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
	if err := rs.runtime.SecondFactor.RevokeCatererPasskey(r.Context(), rs.currentAccountID(r.Context()), passkeyID); err != nil {
		renderSecondFactorFailure(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

// passkeyEnrollmentChallenge mails the code a passkey registration needs.
func (rs *Resource) passkeyEnrollmentChallenge(w http.ResponseWriter, r *http.Request) {
	challenge, err := rs.runtime.SecondFactor.StartCatererPasskeyEnrollment(r.Context(), rs.currentAccountID(r.Context()), common.ParseClientIP(r))
	if err != nil {
		renderSecondFactorFailure(w, r, err)
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
	options, err := rs.runtime.SecondFactor.BeginCatererPasskeyRegistration(r.Context(), identityaccess.CatererPasskeyRegistrationStart{
		AccountID: rs.currentAccountID(r.Context()), ExpectedOrigin: common.PasskeyOrigin(r), Code: req.Code, Name: req.Name,
	})
	if err != nil {
		renderSecondFactorFailure(w, r, err)
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
	summary, err := rs.runtime.SecondFactor.FinishCatererPasskeyRegistration(r.Context(), identityaccess.AccountPasskeyRegistrationFinish{
		AccountID: rs.currentAccountID(r.Context()), SessionID: req.SessionID, CredentialResponse: req.Response, Name: req.Name,
	})
	if err != nil {
		renderSecondFactorFailure(w, r, err)
		return
	}
	render.JSON(w, r, summary)
}
