package traegerportal

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// The second factor of the Träger-Portal (ADR 0084, #4651). The e-mailed code
// and its enrollment are owned by Identity & Access; the composition root
// binds them with the org challenge scope, so a challenge of another portal
// never reaches these handlers. The portal remembers no device: it has no
// school whose trusted-device settings would apply.

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
	// StartChallenge mails the enrollment code and names its row.
	StartChallenge func(ctx context.Context, accountID int64, ip net.IP) (string, error)
	Enroll         func(ctx context.Context, accountID int64) error

	// ChallengeUnusable reports an expired, spent, mis-scoped or wrong code.
	ChallengeUnusable func(error) bool
	// Blocked reports a locked-out or rate-limited second factor.
	Blocked func(error) bool
	// Unavailable reports the fail-closed status lookup.
	Unavailable func(error) bool
	// AlreadyEnrolled reports a repeated enrollment, which is not a failure.
	AlreadyEnrolled func(error) bool
}

func (rt *MFARuntime) complete() bool {
	return rt != nil && rt.VerifyChallenge != nil && rt.VerifyChallengeForOwner != nil &&
		rt.ResendChallenge != nil && rt.StartChallenge != nil && rt.Enroll != nil &&
		rt.ChallengeUnusable != nil && rt.Blocked != nil && rt.Unavailable != nil && rt.AlreadyEnrolled != nil
}

var errMFANotComposed = errors.New("the Träger second factor is not composed")

// MFAVerifyRequest is the body of POST /traeger/auth/mfa/verify and
// /traeger/auth/mfa/enroll/confirm. OrganizationID is the Träger the login
// answered with; zero lets the backend take the only Träger of the account.
type MFAVerifyRequest struct {
	ChallengeToken string `json:"challenge_token"`
	Code           string `json:"code"`
	OrganizationID int64  `json:"organization_id,omitempty"`
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

// EnrollStartResponse carries the challenge the enrollment code belongs to;
// the confirm step redeems exactly that challenge.
type EnrollStartResponse struct {
	ChallengeToken string `json:"challenge_token"`
}

// TokenResponse is the plain token pair of a completed second factor.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
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
		rs.renderMFAError(w, r, err)
		return
	}
	rs.completeTraegerExchange(w, r, verified, req.OrganizationID)
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
		rs.renderMFAError(w, r, err)
		return
	}
	render.JSON(w, r, common.MFAResendResponse{ChallengeToken: renewed})
}

// mfaEnrollStart mails the code that sets up the second factor. The shared
// enrollment authenticator accepts every enrollment scope, so the handler
// pins the org scope.
func (rs *Resource) mfaEnrollStart(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	accountID, ok := traegerEnrollmentAccount(r)
	if !ok {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	challengeToken, err := rs.mfa.StartChallenge(r.Context(), accountID, common.ParseClientIP(r))
	if err != nil {
		rs.renderMFAError(w, r, err)
		return
	}
	render.JSON(w, r, EnrollStartResponse{ChallengeToken: challengeToken})
}

// mfaEnrollConfirm redeems the enrollment code, records the factor and mints
// the Träger session. The challenge is pinned to the account of the
// enrollment token inside the owner, before the code is compared and
// consumed.
func (rs *Resource) mfaEnrollConfirm(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	accountID, ok := traegerEnrollmentAccount(r)
	if !ok {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	req := &MFAVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	verified, err := rs.mfa.VerifyChallengeForOwner(r.Context(), req.ChallengeToken, req.Code, accountID)
	if err != nil {
		rs.renderMFAError(w, r, err)
		return
	}
	if err := rs.mfa.Enroll(r.Context(), accountID); err != nil && !rs.mfa.AlreadyEnrolled(err) {
		rs.renderMFAError(w, r, err)
		return
	}
	rs.completeTraegerExchange(w, r, verified, req.OrganizationID)
}

// traegerEnrollmentAccount reads the account of an org-scope enrollment token.
func traegerEnrollmentAccount(r *http.Request) (int64, bool) {
	claims, ok := sessiontoken.EnrollmentClaimsFromCtx(r.Context())
	if !ok || claims.AccountID == 0 || claims.Scope != sessiontoken.MFAEnrollmentScopeOrg || claims.TenantID != 0 {
		return 0, false
	}
	return claims.AccountID, true
}

// completeTraegerExchange mints the Träger session once the second factor is
// proven, for the verify and the enrollment confirm alike.
func (rs *Resource) completeTraegerExchange(w http.ResponseWriter, r *http.Request, proof VerifiedChallenge, organizationID int64) {
	if !rs.auth.complete() {
		common.RenderError(w, r, common.ErrorInternalServer(errLoginUnavailable))
		return
	}
	result, err := rs.auth.IssueTokens(r.Context(), proof, organizationID, common.GetClientIPString(r), r.Header.Get(headerUserAgent))
	if err != nil {
		if rs.auth.InvalidSession(err) {
			common.RenderError(w, r, common.ErrorUnauthorizedWithCode(errMFACodeInvalid, common.CodeIdentityMfaCodeInvalid))
			return
		}
		rs.renderLoginError(w, r, err)
		return
	}
	render.JSON(w, r, TokenResponse{AccessToken: result.AccessToken, RefreshToken: result.RefreshToken})
}

func (rs *Resource) requireMFA(w http.ResponseWriter, r *http.Request) bool {
	return common.RequireDependency(w, r, rs.mfa.complete(), errMFANotComposed)
}

// renderMFAError maps the second-factor refusals like the school portal.
func (rs *Resource) renderMFAError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case rs.mfa.ChallengeUnusable(err):
		common.RenderError(w, r, common.ErrorUnauthorizedWithCode(errMFACodeInvalid, common.CodeIdentityMfaCodeInvalid))
	case rs.mfa.Blocked(err):
		common.RenderError(w, r, common.ErrorTooManyRequestsWithCode(errMFABlocked, common.CodeIdentityMfaBlocked))
	case rs.mfa.Unavailable(err):
		common.RenderError(w, r, common.ErrorServiceUnavailableWithCode(errMFAUnavailable, common.CodeIdentityMfaStatusUnavailable))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
