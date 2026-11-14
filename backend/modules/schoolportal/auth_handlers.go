package schoolportal

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/inbound/portalauth"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
)

// LoginRequest is the body shape for POST /school/auth/login. TenantSlug is
// optional for direct school-portal logins, and pins an OGS-portal handoff to
// the school the user selected.
type LoginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	TenantSlug string `json:"tenant_slug,omitempty"`
}

// Bind normalizes + validates the request.
func (req *LoginRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.TenantSlug = strings.TrimSpace(req.TenantSlug)

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.Password, validation.Required),
	)
}

// SwitchSchoolRequest is the body shape for POST /school/auth/switch-school.
type SwitchSchoolRequest struct {
	TenantSlug string `json:"tenant_slug"`
}

// Bind validates the switch-school request.
func (req *SwitchSchoolRequest) Bind(_ *http.Request) error {
	req.TenantSlug = strings.TrimSpace(req.TenantSlug)
	return validation.ValidateStruct(req,
		validation.Field(&req.TenantSlug, validation.Required),
	)
}

// login signs a Lehrkraft in to a school-scope session, or answers the
// second factor still to pass.
func (rs *Resource) login(w http.ResponseWriter, r *http.Request) {
	req := &LoginRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !common.RequireDependency(w, r, rs.AuthService.complete(), errLoginUnavailable) {
		return
	}
	client := portalauth.ClientOf(r)
	var result *identityaccess.LoginResult
	var err error
	if req.TenantSlug == "" {
		result, err = rs.AuthService.Login(r.Context(), req.Email, req.Password, client.IP, client.UserAgent, common.TrustedDevice(r))
	} else {
		result, err = rs.AuthService.LoginAtSchool(r.Context(), req.Email, req.Password, client.IP, client.UserAgent, common.TrustedDevice(r), req.TenantSlug)
	}
	if err != nil {
		portalauth.Render(w, r, err, portalauth.LoginRules)
		return
	}
	portalauth.RenderLogin(w, r, result)
}

// mfaVerify redeems a school challenge for a school-scope session. The
// challenge's scope is checked before any code is compared, so a challenge of
// another portal is never consumed here.
func (rs *Resource) mfaVerify(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &common.MFAVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	verified, err := rs.MFAService.VerifyChallenge(r.Context(), req.ChallengeToken, req.Code)
	if err != nil {
		portalauth.Render(w, r, err, portalauth.MFARules)
		return
	}
	rs.completeSchoolExchange(w, r, verified, req.RememberDevice)
}

// completeSchoolExchange mints the school session of a proven second factor
// for the verify and the enrollment confirm alike; the school-portal role is
// checked again.
func (rs *Resource) completeSchoolExchange(w http.ResponseWriter, r *http.Request, proof VerifiedChallenge, rememberDevice bool) {
	if !common.RequireDependency(w, r, rs.AuthService.complete(), errLoginUnavailable) {
		return
	}
	client := portalauth.ClientOf(r)
	prepare := portalauth.RememberDevice(rememberDevice, func(ctx context.Context) (identityaccess.TrustedDeviceIssue, error) {
		return rs.MFAService.PrepareTrustedDevice(ctx, proof.AccountID, proof.TenantID, client.UserAgent, common.ParseClientIP(r))
	})
	tokens, err := portalauth.Exchange(w, r, proof.AccountID, prepare, func(ctx context.Context) (string, string, error) {
		return rs.AuthService.IssueTokens(ctx, proof, client.IP, client.UserAgent)
	})
	if err != nil {
		portalauth.Render(w, r, err, portalauth.ExchangeRules)
		return
	}
	render.JSON(w, r, tokens)
}

// mfaResend mails a new code for a school challenge and answers with the
// renewed challenge token; a challenge of another portal cannot burn its
// budget here.
func (rs *Resource) mfaResend(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &common.MFAResendRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	renewed, err := rs.MFAService.ResendChallenge(r.Context(), req.ChallengeToken, common.ParseClientIP(r))
	if err != nil {
		portalauth.Render(w, r, err, portalauth.MFARules)
		return
	}
	render.JSON(w, r, common.MFAResendResponse{ChallengeToken: renewed})
}

// mfaEnrollStart mails the enrollment code and answers with its challenge;
// the confirm step is bound to exactly that challenge. The route admits only
// a school enrollment token.
func (rs *Resource) mfaEnrollStart(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims, ok := sessiontoken.EnrollmentClaimsFromCtx(r.Context())
	if !ok {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	challengeToken, err := rs.MFAService.StartChallenge(r.Context(), claims.AccountID, claims.TenantID, common.ParseClientIP(r), claims.CredentialGeneration)
	if err != nil {
		portalauth.Render(w, r, err, portalauth.MFARules)
		return
	}
	render.JSON(w, r, portalauth.EnrollStartResponse{ChallengeToken: challengeToken})
}

// mfaEnrollConfirm redeems the enrollment challenge, records the factor and
// mints a school session, so the enrollment detour never turns a school
// login into a tenant session.
//
// The owner redeems the exact challenge the start step minted and pins it to
// the enrollment token's account and school before the code is compared and
// consumed. "The account's newest active code" would let a concurrent
// challenge of another portal or school be consumed here, and a check after
// the consume would let a foreign challenge token burn someone's login.
func (rs *Resource) mfaEnrollConfirm(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) || !common.RequireDependency(w, r, rs.TenantContext != nil, errMFAServiceUnavailable) {
		return
	}
	claims, ok := sessiontoken.EnrollmentClaimsFromCtx(r.Context())
	if !ok {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	req := &common.MFAVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	// The enrollment route runs outside the tenant transaction; the claims'
	// school carries the MFA audit events into audit.auth_events.
	ctx := rs.TenantContext(r.Context(), claims.TenantID)
	verified, err := rs.MFAService.VerifyChallengeForOwner(ctx, req.ChallengeToken, req.Code, claims.AccountID, claims.TenantID)
	if err != nil {
		portalauth.Render(w, r, err, portalauth.MFARules)
		return
	}
	// The owner already refused a challenge of another account or school;
	// the re-check keeps the handler honest if that binding ever moves.
	if verified.AccountID != claims.AccountID || verified.TenantID != claims.TenantID {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	// A retried confirm must still produce a session. The enrollment holds
	// the redeemed code's credential generation (#6518).
	if err := rs.MFAService.Enroll(ctx, verified); err != nil && !errors.Is(err, identityaccess.ErrMFAAlreadyEnrolled) {
		portalauth.Render(w, r, err, portalauth.MFARules)
		return
	}
	rs.completeSchoolExchange(w, r, verified, req.RememberDevice)
}

// switchSchool moves a school session to another school where the account
// holds a school-portal role.
func (rs *Resource) switchSchool(w http.ResponseWriter, r *http.Request) {
	req := &SwitchSchoolRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !common.RequireDependency(w, r, rs.AuthService.complete(), errLoginUnavailable) {
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	client := portalauth.ClientOf(r)
	accessToken, refreshToken, err := rs.AuthService.SwitchSchool(r.Context(), int64(claims.ID), req.TenantSlug, client.IP, client.UserAgent, claims.FamilyID)
	if err != nil {
		portalauth.Render(w, r, err, portalauth.SwitchRules)
		return
	}
	render.JSON(w, r, portalauth.TokenResponse{AccessToken: accessToken, RefreshToken: refreshToken})
}

// listTrustedDevices answers the Lehrkraft's remembered devices at the school
// of the session. A device the tenant portal remembered for the same account
// at the same school is the same row: each portal has its own host and
// cookie jar, so the list shows what can open either at this school.
func (rs *Resource) listTrustedDevices(w http.ResponseWriter, r *http.Request) {
	claims, ok := rs.deviceClaims(w, r)
	if !ok {
		return
	}
	portalauth.ListTrustedDevices(w, r, func(ctx context.Context) ([]identityaccess.AccountTrustedDevice, error) {
		return rs.MFAService.ListTrustedDevices(ctx, int64(claims.ID), claims.TenantID)
	}, portalauth.RefuseMFA)
}

// revokeTrustedDevice forgets one remembered device of the Lehrkraft at the
// school of the session; the next sign-in on it asks for the code again.
func (rs *Resource) revokeTrustedDevice(w http.ResponseWriter, r *http.Request) {
	claims, ok := rs.deviceClaims(w, r)
	if !ok {
		return
	}
	portalauth.RevokeTrustedDevice(w, r, func(ctx context.Context, deviceID int64) error {
		return rs.MFAService.RevokeTrustedDevice(ctx, int64(claims.ID), claims.TenantID, deviceID)
	}, portalauth.RefuseMFA)
}

func (rs *Resource) deviceClaims(w http.ResponseWriter, r *http.Request) (sessiontoken.AppClaims, bool) {
	if !common.RequireDependency(w, r, rs.MFAService.devicesComplete(), errMFAServiceUnavailable) {
		return sessiontoken.AppClaims{}, false
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 || claims.TenantID <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return sessiontoken.AppClaims{}, false
	}
	return claims, true
}

func (rs *Resource) requireMFA(w http.ResponseWriter, r *http.Request) bool {
	return common.RequireDependency(w, r, rs.MFAService.complete(), errMFAServiceUnavailable)
}
