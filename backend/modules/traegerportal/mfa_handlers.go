package traegerportal

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

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
// never reaches these handlers. When the person asks for it at the code step,
// the portal remembers the device at the Träger of the session (#6955), with
// the registry defaults of the trusted-device settings: a Träger has no
// school whose settings would apply, like the caterer portal (#4497).

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
	// PrepareTrustedDevice freezes the remember-device policy before the
	// mint; the operation it returns stores the device at the Träger its
	// context binds (identityaccess.WithTraegerDevice). Nil remembers none.
	PrepareTrustedDevice func(ctx context.Context, accountID int64, userAgent string, ip net.IP) (identityaccess.TrustedDeviceIssue, error)
	// ListTrustedDevices and RevokeTrustedDevice reach only the account's
	// devices at that Träger.
	ListTrustedDevices  func(ctx context.Context, accountID, organizationID int64) ([]identityaccess.AccountTrustedDevice, error)
	RevokeTrustedDevice func(ctx context.Context, accountID, organizationID, deviceID int64) error
}

func (rt *MFARuntime) complete() bool {
	return rt != nil && rt.VerifyChallenge != nil && rt.VerifyChallengeForOwner != nil &&
		rt.ResendChallenge != nil && rt.StartChallenge != nil && rt.Enroll != nil
}

func (rt *MFARuntime) devicesComplete() bool {
	return rt != nil && rt.ListTrustedDevices != nil && rt.RevokeTrustedDevice != nil
}

var (
	errMFANotComposed       = errors.New("the Träger second factor is not composed")
	errDeviceWithoutTraeger = errors.New("the Träger session names no Träger to remember the device at")
	errDevicesNotComposed   = errors.New("the Träger remembered devices are not composed")
)

// MFAVerifyRequest is the body of POST /traeger/auth/mfa/verify and
// /traeger/auth/mfa/enroll/confirm. OrganizationID is the Träger the login
// answered with; zero lets the backend take the only Träger of the account.
// RememberDevice asks to remember the browser at that Träger (#6955); the
// enrollment confirm ignores it.
type MFAVerifyRequest struct {
	ChallengeToken string `json:"challenge_token"`
	Code           string `json:"code"`
	OrganizationID int64  `json:"organization_id,omitempty"`
	RememberDevice bool   `json:"remember_device,omitempty"`
}

// Bind validates the request like the other portals' verify body.
func (req *MFAVerifyRequest) Bind(r *http.Request) error {
	shared := common.MFAVerifyRequest{ChallengeToken: req.ChallengeToken, Code: req.Code}
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
	rs.completeTraegerExchange(w, r, verified, req.OrganizationID, req.RememberDevice)
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
	// Setting the factor up remembers no device: the frontend offers the
	// choice only at the code step of a sign-in.
	rs.completeTraegerExchange(w, r, verified, req.OrganizationID, false)
}

// completeTraegerExchange mints the Träger session once the second factor is
// proven, for the verify and the enrollment confirm alike, in the order every
// portal keeps (portalauth.Exchange): prepare the device, mint, then store
// the device at the Träger the mint resolved and set its cookie. A device
// that cannot be remembered never fails the sign-in.
func (rs *Resource) completeTraegerExchange(w http.ResponseWriter, r *http.Request, proof VerifiedChallenge, organizationID int64, rememberDevice bool) {
	if !common.RequireDependency(w, r, rs.auth.complete(), errLoginUnavailable) {
		return
	}
	client := portalauth.ClientOf(r)
	var mintedAt int64
	prepare := portalauth.RememberDevice(rememberDevice && rs.mfa.PrepareTrustedDevice != nil, func(ctx context.Context) (identityaccess.TrustedDeviceIssue, error) {
		issue, err := rs.mfa.PrepareTrustedDevice(ctx, proof.AccountID, client.UserAgent, common.ParseClientIP(r))
		if err != nil || issue == nil {
			return nil, err
		}
		return func(issueCtx context.Context) (string, time.Time, error) {
			if mintedAt <= 0 {
				return "", time.Time{}, errDeviceWithoutTraeger
			}
			return issue(identityaccess.WithTraegerDevice(issueCtx, mintedAt))
		}, nil
	})
	tokens, err := portalauth.Exchange(w, r, proof.AccountID, prepare, func(ctx context.Context) (string, string, error) {
		result, err := rs.auth.IssueTokens(ctx, proof, organizationID, client.IP, client.UserAgent)
		if err != nil {
			return "", "", err
		}
		mintedAt = result.OrganizationID
		return result.AccessToken, result.RefreshToken, nil
	})
	if err != nil {
		portalauth.Render(w, r, err, portalauth.ExchangeRules, portalauth.LoginRules)
		return
	}
	render.JSON(w, r, tokens)
}

// listTrustedDevices answers the person's remembered devices at the Träger
// of the session.
func (rs *Resource) listTrustedDevices(w http.ResponseWriter, r *http.Request) {
	actor, ok := rs.deviceActor(w, r)
	if !ok {
		return
	}
	portalauth.ListTrustedDevices(w, r, func(ctx context.Context) ([]identityaccess.AccountTrustedDevice, error) {
		return rs.mfa.ListTrustedDevices(ctx, actor.AccountID, actor.OrganizationID)
	}, portalauth.RefuseMFA)
}

// revokeTrustedDevice forgets one remembered device of the person at the
// Träger of the session; the next sign-in on it asks for the code again.
func (rs *Resource) revokeTrustedDevice(w http.ResponseWriter, r *http.Request) {
	actor, ok := rs.deviceActor(w, r)
	if !ok {
		return
	}
	portalauth.RevokeTrustedDevice(w, r, func(ctx context.Context, deviceID int64) error {
		return rs.mfa.RevokeTrustedDevice(ctx, actor.AccountID, actor.OrganizationID, deviceID)
	}, portalauth.RefuseMFA)
}

func (rs *Resource) deviceActor(w http.ResponseWriter, r *http.Request) (identityaccess.TraegerActor, bool) {
	if !common.RequireDependency(w, r, rs.mfa.devicesComplete(), errDevicesNotComposed) {
		return identityaccess.TraegerActor{}, false
	}
	return sessionActor(w, r)
}

func (rs *Resource) requireMFA(w http.ResponseWriter, r *http.Request) bool {
	return common.RequireDependency(w, r, rs.mfa.complete(), errMFANotComposed)
}
