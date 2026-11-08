package catererauth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/portalauth"
)

var errLoginFieldsRequired = errors.New("email and password are required")

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse is a caterer's password sign-in: the session, or, where the
// caterer switched its second factor on, the challenge of the mailed code
// (#4497).
type LoginResponse struct {
	Status               string `json:"status"`
	AccessToken          string `json:"access_token,omitempty"`
	RefreshToken         string `json:"refresh_token,omitempty"`
	ChallengeToken       string `json:"challenge_token,omitempty"`
	MaskedEmail          string `json:"masked_email,omitempty"`
	TrustedDeviceEnabled bool   `json:"trusted_device_enabled,omitempty"`
	TrustedDeviceDays    int    `json:"trusted_device_days,omitempty"`
}

// login signs a caterer in to the caterer portal. A caterer that switched
// its second factor on gets the challenge of a mailed code, unless the
// browser presents a device it remembered.
func (rs *Resource) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	if request.Email == "" || request.Password == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errLoginFieldsRequired))
		return
	}
	result, err := rs.signIn.LoginCaterer(r.Context(), identityaccess.CatererLoginRequest{
		Email: request.Email, Password: request.Password, IPAddress: common.GetClientIPString(r),
		UserAgent: r.UserAgent(), TrustedDeviceCookie: common.TrustedDevice(r),
	})
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, LoginResponse{
		Status: string(result.Status), AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
		ChallengeToken: result.ChallengeToken, MaskedEmail: result.MaskedEmail,
		TrustedDeviceEnabled: result.TrustedDeviceDays > 0, TrustedDeviceDays: result.TrustedDeviceDays,
	}, "Login successful")
}

// verifyMFA redeems the mailed code of a sign-in.
func (rs *Resource) verifyMFA(w http.ResponseWriter, r *http.Request) {
	req := &common.MFAVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	session, err := rs.signIn.VerifyCatererMFA(r.Context(), identityaccess.CatererMFAVerification{
		ChallengeToken: req.ChallengeToken, Code: req.Code, RememberDevice: req.RememberDevice,
		IPAddress: common.GetClientIPString(r), UserAgent: r.UserAgent(),
	})
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	common.SetTrustedDevice(w, session.TrustedDeviceCookie, session.TrustedDeviceExpiresAt)
	render.JSON(w, r, portalauth.TokenResponse{AccessToken: session.AccessToken, RefreshToken: session.RefreshToken})
}

// resendMFA mails a new code for a caterer challenge.
func (rs *Resource) resendMFA(w http.ResponseWriter, r *http.Request) {
	req := &common.MFAResendRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	renewed, err := rs.signIn.ResendCatererMFA(r.Context(), req.ChallengeToken, common.ParseClientIP(r))
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	render.JSON(w, r, common.MFAResendResponse{ChallengeToken: renewed})
}

// passkeyLoginOptions starts a passkey sign-in on the caterer portal.
func (rs *Resource) passkeyLoginOptions(w http.ResponseWriter, r *http.Request) {
	options, err := rs.signIn.BeginCatererPasskeyLogin(r.Context(), common.PasskeyOrigin(r))
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
	result, err := rs.signIn.FinishCatererPasskeyLogin(r.Context(), identityaccess.AccountPasskeyLoginFinish{
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
