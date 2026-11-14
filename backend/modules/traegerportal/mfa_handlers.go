package traegerportal

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/inbound/portalauth"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
)

// The second factor of the Träger-Portal (ADR 0084, #4651). The e-mailed code
// and its enrollment are owned by Identity & Access; the composition root
// binds them with the org challenge scope, so a challenge of another portal
// never reaches these handlers. A device the person chooses to remember
// counts only in the Träger-Portal and only at the Träger of the sign-in
// (#6955); its lifetime is the registry default.

// VerifiedChallenge is what a redeemed Träger challenge hands the portal.
type VerifiedChallenge struct {
	MFAResetGeneration int64
	AccountID          int64
}

// MFARuntime is the Träger-Portal's second-factor port.
type MFARuntime struct {
	VerifyChallenge func(ctx context.Context, challengeToken, code string) (VerifiedChallenge, error)
	// VerifyChallengeForOwner additionally pins the challenge to the account
	// the enrollment token names, before any code is compared.
	VerifyChallengeForOwner func(ctx context.Context, challengeToken, code string, accountID int64) (VerifiedChallenge, error)
	// ResendChallenge re-issues the code and answers with the renewed JWT.
	ResendChallenge func(ctx context.Context, challengeToken string, ip net.IP) (string, error)
	// StartChallenge mails the enrollment code and names its row; it is bound
	// to the credential generation the enrollment token names (#6289).
	StartChallenge func(ctx context.Context, accountID int64, ip net.IP, credentialGeneration int64) (string, error)
	Enroll         func(ctx context.Context, proof VerifiedChallenge) error
	// PrepareTrustedDevice, ListTrustedDevices and RevokeTrustedDevice act on
	// the devices remembered at one Träger (#6955).
	PrepareTrustedDevice func(ctx context.Context, accountID, organizationID int64, userAgent string, ip net.IP) (identityaccess.TrustedDeviceIssue, error)
	ListTrustedDevices   func(ctx context.Context, accountID, organizationID int64) ([]identityaccess.AccountTrustedDevice, error)
	RevokeTrustedDevice  func(ctx context.Context, accountID, organizationID, deviceID int64) error
}

func (rt *MFARuntime) complete() bool {
	return rt != nil && rt.VerifyChallenge != nil && rt.VerifyChallengeForOwner != nil &&
		rt.ResendChallenge != nil && rt.StartChallenge != nil && rt.Enroll != nil
}

func (rt *MFARuntime) devicesComplete() bool {
	return rt != nil && rt.PrepareTrustedDevice != nil && rt.ListTrustedDevices != nil && rt.RevokeTrustedDevice != nil
}

var (
	errMFANotComposed     = errors.New("the Träger second factor is not composed")
	errDevicesNotComposed = errors.New("the Träger's remembered devices are not composed")
)

// MFAVerifyRequest is the body of POST /traeger/auth/mfa/verify and
// /traeger/auth/mfa/enroll/confirm. OrganizationID is the Träger the login
// answered with; zero lets the backend take the only Träger of the account.
// RememberDevice remembers the browser at that Träger; it needs the Träger
// named, so a request without one remembers nothing.
type MFAVerifyRequest struct {
	ChallengeToken string `json:"challenge_token"`
	Code           string `json:"code"`
	OrganizationID int64  `json:"organization_id,omitempty"`
	RememberDevice bool   `json:"remember_device,omitempty"`
}

// Bind validates the request like the other portals' verify body.
func (req *MFAVerifyRequest) Bind(r *http.Request) error {
	shared := common.MFAVerifyRequest{ChallengeToken: req.ChallengeToken, Code: req.Code, RememberDevice: req.RememberDevice}
	if err := shared.Bind(r); err != nil {
		return err
	}
	req.ChallengeToken, req.Code = shared.ChallengeToken, shared.Code
	return validation.ValidateStruct(req, validation.Field(&req.OrganizationID, validation.Min(int64(0))))
}

// mfaVerify exchanges an org-scope challenge and its code for a Träger
// session. The scope is checked before any code is compared, so a challenge
// of another portal is never consumed here.
func (rs *Resource) mfaVerify(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &MFAVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	verified, err := rs.mfa.VerifyChallenge(r.Context(), req.ChallengeToken, req.Code)
	if err != nil {
		portalauth.Render(w, r, err, portalauth.MFARules)
		return
	}
	rs.completeTraegerExchange(w, r, verified, req)
}

// mfaResend mails a new code for the challenge and answers with the renewed
// challenge token. The rate limit lives in the owner.
func (rs *Resource) mfaResend(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &common.MFAResendRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	renewed, err := rs.mfa.ResendChallenge(r.Context(), req.ChallengeToken, common.ParseClientIP(r))
	if err != nil {
		portalauth.Render(w, r, err, portalauth.MFARules)
		return
	}
	render.JSON(w, r, common.MFAResendResponse{ChallengeToken: renewed})
}

// mfaEnrollStart mails the code that sets up the second factor.
func (rs *Resource) mfaEnrollStart(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims, ok := sessiontoken.EnrollmentClaimsFromCtx(r.Context())
	if !ok {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	accountID := claims.AccountID
	challengeToken, err := rs.mfa.StartChallenge(r.Context(), accountID, common.ParseClientIP(r), claims.CredentialGeneration)
	if err != nil {
		portalauth.Render(w, r, err, portalauth.MFARules)
		return
	}
	render.JSON(w, r, portalauth.EnrollStartResponse{ChallengeToken: challengeToken})
}

// mfaEnrollConfirm redeems the enrollment code, records the factor and mints
// the Träger session. The challenge is pinned to the account of the
// enrollment token inside the owner, before the code is compared and
// consumed.
func (rs *Resource) mfaEnrollConfirm(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims, ok := sessiontoken.EnrollmentClaimsFromCtx(r.Context())
	if !ok {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	accountID := claims.AccountID
	req := &MFAVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	verified, err := rs.mfa.VerifyChallengeForOwner(r.Context(), req.ChallengeToken, req.Code, accountID)
	if err != nil {
		portalauth.Render(w, r, err, portalauth.MFARules)
		return
	}
	// The enrollment holds the redeemed code's credential generation (#6518).
	if err := rs.mfa.Enroll(r.Context(), verified); err != nil && !errors.Is(err, identityaccess.ErrMFAAlreadyEnrolled) {
		portalauth.Render(w, r, err, portalauth.MFARules)
		return
	}
	rs.completeTraegerExchange(w, r, verified, req)
}

// completeTraegerExchange mints the Träger session once the second factor is
// proven, for the verify and the enrollment confirm alike, in the order every
// portal keeps (portalauth.Exchange). The device is remembered only when the
// person chose it and named the Träger: it counts at that Träger alone, and
// the mint checks the membership there.
func (rs *Resource) completeTraegerExchange(w http.ResponseWriter, r *http.Request, proof VerifiedChallenge, req *MFAVerifyRequest) {
	if !common.RequireDependency(w, r, rs.auth.complete(), errLoginUnavailable) {
		return
	}
	client := portalauth.ClientOf(r)
	remember := req.RememberDevice && req.OrganizationID > 0 && rs.mfa.devicesComplete()
	prepare := portalauth.RememberDevice(remember, func(ctx context.Context) (identityaccess.TrustedDeviceIssue, error) {
		return rs.mfa.PrepareTrustedDevice(ctx, proof.AccountID, req.OrganizationID, client.UserAgent, common.ParseClientIP(r))
	})
	tokens, err := portalauth.Exchange(w, r, proof.AccountID, prepare, func(ctx context.Context) (string, string, error) {
		result, err := rs.auth.IssueTokens(ctx, proof, req.OrganizationID, client.IP, client.UserAgent)
		if err != nil {
			return "", "", err
		}
		return result.AccessToken, result.RefreshToken, nil
	})
	if err != nil {
		portalauth.Render(w, r, err, portalauth.ExchangeRules, portalauth.LoginRules)
		return
	}
	render.JSON(w, r, tokens)
}

// mfaTrustedDevices lists the devices the signed-in person remembered at the
// Träger of the session.
func (rs *Resource) mfaTrustedDevices(w http.ResponseWriter, r *http.Request) {
	if !common.RequireDependency(w, r, rs.mfa.devicesComplete(), errDevicesNotComposed) {
		return
	}
	actor, ok := sessionActor(w, r)
	if !ok {
		return
	}
	devices, err := rs.mfa.ListTrustedDevices(r.Context(), actor.AccountID, actor.OrganizationID)
	if err != nil {
		portalauth.Render(w, r, err, portalauth.MFARules)
		return
	}
	render.JSON(w, r, common.MapTrustedDevices(devices, portalauth.TrustedDeviceRowOf))
}

// mfaRevokeTrustedDevice forgets one device of the signed-in person at the
// Träger of the session; a device of another account, portal or Träger
// answers 403.
func (rs *Resource) mfaRevokeTrustedDevice(w http.ResponseWriter, r *http.Request) {
	if !common.RequireDependency(w, r, rs.mfa.devicesComplete(), errDevicesNotComposed) {
		return
	}
	actor, ok := sessionActor(w, r)
	if !ok {
		return
	}
	deviceID, err := common.ParseIDParam(r, "deviceId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.mfa.RevokeTrustedDevice(r.Context(), actor.AccountID, actor.OrganizationID, deviceID); err != nil {
		portalauth.Render(w, r, err, portalauth.MFARules)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (rs *Resource) requireMFA(w http.ResponseWriter, r *http.Request) bool {
	return common.RequireDependency(w, r, rs.mfa.complete(), errMFANotComposed)
}
