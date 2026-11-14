// Package parentauth serves the parents portal's sign-in with the account's
// second factor and the parent's own management of it (#7051, ADR 0216
// addendum). The parents portal mounts these routes into its /parent
// router. They answer through the sign-in tables of portalauth, which every
// account portal shares (#6172), and in the shapes of the caterer portal's
// routes, so the portals share their frontend.
package parentauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/inbound/portalauth"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
)

// Resource serves the parents' sign-in and second factor.
type Resource struct {
	signIn       identityaccess.ParentSignIn
	secondFactor identityaccess.ParentSecondFactor
}

// NewResource builds the resource; both flows are required.
func NewResource(access *identityaccess.ParentAccess) *Resource {
	if access == nil || access.ParentSignIn == nil || access.ParentSecondFactor == nil {
		panic("parent auth HTTP: sign-in and second factor are required")
	}
	return &Resource{signIn: access.ParentSignIn, secondFactor: access.ParentSecondFactor}
}

// MountSignIn mounts the public sign-in routes under the portal's /auth
// prefix: the password, the code of the second factor and its resend. They
// check credentials, so the caller puts them behind the auth rate limit.
// after runs once a sign-in minted its session, with the signed-in address;
// the parents portal claims the account's holiday registrations with it
// (ADR 0216). It logs its own failures; the sign-in stands.
func (rs *Resource) MountSignIn(r chi.Router, after func(ctx context.Context, email string)) {
	if after == nil {
		after = func(context.Context, string) {}
	}
	r.Post("/login", func(w http.ResponseWriter, r *http.Request) { rs.login(w, r, after) })
	r.Post("/mfa/verify", func(w http.ResponseWriter, r *http.Request) { rs.verifyMFA(w, r, after) })
	r.Post("/mfa/resend", rs.resendMFA)
}

// MountSession mounts the routes of a parent session that read or revoke
// the factor's state.
func (rs *Resource) MountSession(r chi.Router) {
	r.Get("/me/mfa", rs.mfaStatus)
	r.Get("/me/mfa/trusted-devices", rs.listTrustedDevices)
	r.Delete("/me/mfa/trusted-devices/{"+portalauth.TrustedDeviceIDParam+"}", rs.revokeTrustedDevice)
}

// MountCredentialChecks mounts the session routes that mail a code or check
// one or the current password. The caller puts them behind the auth rate
// limit.
func (rs *Resource) MountCredentialChecks(r chi.Router) {
	r.Post("/me/mfa/start", rs.startMFA)
	r.Post("/me/mfa/confirm", rs.confirmMFA)
	r.Post("/me/mfa/disable", rs.disableMFA)
}

func currentAccountID(ctx context.Context) int64 {
	return int64(sessiontoken.ClaimsFromCtx(ctx).ID)
}

var errLoginFieldsRequired = errors.New("email and password are required")

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// login signs a parent in. An account with a second factor gets the
// challenge of a mailed code, unless the browser presents a device the
// parents portal remembered.
func (rs *Resource) login(w http.ResponseWriter, r *http.Request, after func(context.Context, string)) {
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
	if err := validation.Validate(request.Email, is.Email); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	client := portalauth.ClientOf(r)
	result, err := rs.signIn.LoginParent(r.Context(), identityaccess.ParentLoginRequest{
		Email: request.Email, Password: request.Password, IPAddress: client.IP,
		UserAgent: client.UserAgent, TrustedDeviceCookie: common.TrustedDevice(r),
	})
	if err != nil {
		portalauth.Render(w, r, err, signInRules)
		return
	}
	if result.Status == identityaccess.LoginStatusAuthenticated {
		after(r.Context(), request.Email)
	}
	body := portalauth.LoginResponse{
		Status: string(result.Status), AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
		ChallengeToken: result.ChallengeToken, MaskedEmail: result.MaskedEmail,
	}
	if result.Status == identityaccess.LoginStatusMFARequired {
		enabled, days := result.TrustedDeviceDays > 0, result.TrustedDeviceDays
		body.TrustedDeviceEnabled, body.TrustedDeviceDays = &enabled, &days
	}
	common.Respond(w, r, http.StatusOK, body, "Login successful")
}

// verifyMFA redeems the mailed code of a sign-in.
func (rs *Resource) verifyMFA(w http.ResponseWriter, r *http.Request, after func(context.Context, string)) {
	req := &common.MFAVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	client := portalauth.ClientOf(r)
	session, err := rs.signIn.VerifyParentMFA(r.Context(), identityaccess.ParentMFAVerification{
		ChallengeToken: req.ChallengeToken, Code: req.Code, RememberDevice: req.RememberDevice,
		IPAddress: client.IP, UserAgent: client.UserAgent,
	})
	if err != nil {
		portalauth.Render(w, r, err, signInRules)
		return
	}
	after(r.Context(), session.Email)
	common.SetTrustedDevice(w, session.TrustedDeviceCookie, session.TrustedDeviceExpiresAt)
	render.JSON(w, r, portalauth.TokenResponse{AccessToken: session.AccessToken, RefreshToken: session.RefreshToken})
}

// resendMFA mails a new code for a parents-portal challenge.
func (rs *Resource) resendMFA(w http.ResponseWriter, r *http.Request) {
	req := &common.MFAResendRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	renewed, err := rs.signIn.ResendParentMFA(r.Context(), req.ChallengeToken, common.ParseClientIP(r))
	if err != nil {
		portalauth.Render(w, r, err, signInRules)
		return
	}
	render.JSON(w, r, common.MFAResendResponse{ChallengeToken: renewed})
}

// MFAStatusResponse is the second factor as the portal's settings show it.
// Available is false for an account that is a parent at no school: its
// factor stays off.
type MFAStatusResponse struct {
	Enabled     bool   `json:"enabled"`
	Available   bool   `json:"available"`
	MaskedEmail string `json:"masked_email"`
}

// MFAChallengeResponse names the code mailed to switch the factor on.
type MFAChallengeResponse struct {
	ChallengeToken string `json:"challenge_token"`
	MaskedEmail    string `json:"masked_email"`
}

func (rs *Resource) mfaStatus(w http.ResponseWriter, r *http.Request) {
	state, err := rs.secondFactor.ParentMFAStatus(r.Context(), currentAccountID(r.Context()))
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	render.JSON(w, r, MFAStatusResponse(state))
}

// startMFA mails the code that switches the second factor on.
func (rs *Resource) startMFA(w http.ResponseWriter, r *http.Request) {
	challenge, err := rs.secondFactor.StartParentMFA(r.Context(), currentAccountID(r.Context()), common.ParseClientIP(r))
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	render.JSON(w, r, MFAChallengeResponse(challenge))
}

// confirmMFA switches the second factor on and answers with a session
// proven by it; the account's older sessions end at their next refresh.
func (rs *Resource) confirmMFA(w http.ResponseWriter, r *http.Request) {
	verify := &common.MFAVerifyRequest{}
	if err := render.Bind(r, verify); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	client := portalauth.ClientOf(r)
	session, err := rs.secondFactor.ConfirmParentMFA(r.Context(), identityaccess.ParentMFAConfirmation{
		AccountID: currentAccountID(r.Context()), ChallengeToken: verify.ChallengeToken, Code: verify.Code,
		IPAddress: client.IP, UserAgent: client.UserAgent,
	})
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	render.JSON(w, r, portalauth.TokenResponse{AccessToken: session.AccessToken, RefreshToken: session.RefreshToken})
}

func (rs *Resource) disableMFA(w http.ResponseWriter, r *http.Request) {
	portalauth.SwitchOffFactor(w, r, func(ctx context.Context, currentPassword string, client portalauth.Client) error {
		return rs.secondFactor.DisableParentMFA(ctx, currentAccountID(ctx), currentPassword, client.IP, client.UserAgent)
	}, renderFailure)
}

func (rs *Resource) listTrustedDevices(w http.ResponseWriter, r *http.Request) {
	portalauth.ListTrustedDevices(w, r, func(ctx context.Context) ([]identityaccess.AccountTrustedDevice, error) {
		return rs.secondFactor.ListParentTrustedDevices(ctx, currentAccountID(ctx))
	}, renderFailure)
}

func (rs *Resource) revokeTrustedDevice(w http.ResponseWriter, r *http.Request) {
	portalauth.RevokeTrustedDevice(w, r, func(ctx context.Context, deviceID int64) error {
		return rs.secondFactor.RevokeParentTrustedDevice(ctx, currentAccountID(ctx), deviceID)
	}, renderFailure)
}
