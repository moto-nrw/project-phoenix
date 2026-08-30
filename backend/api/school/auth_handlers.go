package school

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const headerUserAgent = "User-Agent"

// trustedDeviceCookieName matches the tenant portal's cookie name — the
// trusted-device record itself is keyed by (account, tenant) in the service,
// so a device marked on one portal is legitimately trusted on the other for
// the same account + school.
const trustedDeviceCookieName = "mfa_trust_device"

var errMFAServiceUnavailable = errors.New("mfa service is not configured for this deployment")

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

// LoginResponse mirrors the tenant login response shape (status +
// tokens / challenge) so the frontend NextAuth provider can share the
// MFA-aware consumption logic.
type LoginResponse struct {
	Status                string `json:"status"`
	AccessToken           string `json:"access_token,omitempty"`
	RefreshToken          string `json:"refresh_token,omitempty"`
	ChallengeToken        string `json:"challenge_token,omitempty"`
	MaskedEmail           string `json:"masked_email,omitempty"`
	MFAEnrollmentRequired bool   `json:"mfa_enrollment_required,omitempty"`
	TrustedDeviceEnabled  *bool  `json:"trusted_device_enabled,omitempty"`
	TrustedDeviceDays     *int   `json:"trusted_device_days,omitempty"`
}

// TokenResponse is the plain token-pair shape — same as the tenant and
// parent login responses.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
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

// login authenticates a school-portal user via email/password and issues a
// school-scope JWT (or an MFA challenge). Refuses accounts without a
// school-portal role at any school.
func (rs *Resource) login(w http.ResponseWriter, r *http.Request) {
	req := &LoginRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	ipAddress := common.GetClientIPString(r)
	userAgent := r.Header.Get(headerUserAgent)

	var trustedDeviceCookie string
	if c, err := r.Cookie(trustedDeviceCookieName); err == nil {
		trustedDeviceCookie = c.Value
	}

	var result *authService.LoginResult
	var err error
	if req.TenantSlug == "" {
		result, err = rs.AuthService.LoginSchoolWithMFAGate(
			r.Context(), req.Email, req.Password, ipAddress, userAgent, trustedDeviceCookie,
		)
	} else {
		result, err = rs.AuthService.LoginSchoolAtTenantWithMFAGate(
			r.Context(), req.Email, req.Password, ipAddress, userAgent, trustedDeviceCookie, req.TenantSlug,
		)
	}
	if err != nil {
		rs.handleLoginError(w, r, err)
		return
	}

	switch result.Status {
	case authService.LoginStatusMFARequired:
		tde := result.TrustedDeviceEnabled
		tdd := result.TrustedDeviceDays
		render.JSON(w, r, LoginResponse{
			Status:               string(authService.LoginStatusMFARequired),
			ChallengeToken:       result.ChallengeToken,
			MaskedEmail:          result.MaskedEmail,
			TrustedDeviceEnabled: &tde,
			TrustedDeviceDays:    &tdd,
		})
		return
	case authService.LoginStatusMFAEnrollmentRequired:
		render.JSON(w, r, LoginResponse{
			Status:                string(authService.LoginStatusMFAEnrollmentRequired),
			AccessToken:           result.AccessToken,
			MaskedEmail:           result.MaskedEmail,
			MFAEnrollmentRequired: true,
		})
		return
	}

	render.JSON(w, r, LoginResponse{
		Status:       string(authService.LoginStatusAuthenticated),
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	})
}

// handleLoginError maps school-login service errors onto HTTP responses.
// Credential-shaped failures mirror the parent login (enumeration-safe);
// the portal-role refusal gets a stable code the frontend switches on.
func (rs *Resource) handleLoginError(w http.ResponseWriter, r *http.Request, err error) {
	var authErr *authService.AuthError
	if errors.As(err, &authErr) {
		switch {
		case errors.Is(authErr.Err, authService.ErrInvalidCredentials),
			errors.Is(authErr.Err, authService.ErrAccountNotFound):
			// Mask the specific cause to prevent account enumeration.
			common.RenderError(w, r, common.ErrorUnauthorizedWithCode(
				authService.ErrInvalidCredentials, "invalid_credentials"))
		case errors.Is(authErr.Err, authService.ErrAccountInactive):
			common.RenderError(w, r, common.ErrorUnauthorizedWithCode(
				authService.ErrAccountInactive, "account_inactive"))
		case errors.Is(authErr.Err, authService.ErrAccountNoSchoolPortalRole):
			// 403 with a stable code — reachable only after the password
			// was accepted, so it leaks nothing about foreign accounts.
			common.RenderError(w, r, common.ErrorForbiddenWithCode(
				authService.ErrAccountNoSchoolPortalRole, "no_school_portal_role"))
		case errors.Is(authErr.Err, authService.ErrTenantNotFound):
			// The pinned school is deactivated or deleted — same 404 the
			// switch-school path returns for that state.
			common.RenderError(w, r, common.ErrorNotFound(authService.ErrTenantNotFound))
		case errors.Is(authErr.Err, authService.ErrTenantAccessDenied):
			common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTenantAccessDenied))
		case errors.Is(authErr.Err, authService.ErrMFARateLimited),
			errors.Is(authErr.Err, authService.ErrMFALocked):
			common.RenderError(w, r, common.ErrorTooManyRequests(authErr.Err))
		case errors.Is(authErr.Err, authService.ErrMFAStatusUnavailable):
			// Fail-closed MFA status lookup — 503 so the client retries
			// instead of treating it as bad credentials.
			common.RenderError(w, r, common.ErrorServiceUnavailable(authErr.Err))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
}

// mfaVerify exchanges a SCHOOL-scope challenge token + email code for a
// school-scope token pair. Challenges started at the tenant login carry the
// tenant scope and are refused here (and vice versa) — the scope check
// happens before any code comparison, so a mismatched challenge is never
// consumed.
func (rs *Resource) mfaVerify(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &common.MFAVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	verified, err := rs.MFAService.VerifyChallengeForScope(r.Context(), req.ChallengeToken, req.Code, jwt.MFAChallengeScopeSchool)
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	if verified == nil {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}

	rs.completeSchoolExchange(w, r, verified.AccountID, verified.TenantID, req.RememberDevice)
}

// completeSchoolExchange mints the school-scope token pair once the second
// factor is proven — shared by the challenge verify and the enrollment
// confirm, so the role re-check, error mapping, and trusted-device cookie
// stay in one place.
func (rs *Resource) completeSchoolExchange(w http.ResponseWriter, r *http.Request, accountID, tenantID int64, rememberDevice bool) {
	ipAddress := common.GetClientIPString(r)
	userAgent := r.Header.Get(headerUserAgent)

	accessToken, refreshToken, err := rs.AuthService.IssueSchoolTokensForAuthenticatedAccount(
		r.Context(), accountID, tenantID, ipAddress, userAgent,
	)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrInvalidCredentials))
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountInactive))
			case errors.Is(authErr.Err, authService.ErrAccountNoSchoolPortalRole):
				common.RenderError(w, r, common.ErrorForbiddenWithCode(
					authService.ErrAccountNoSchoolPortalRole, "no_school_portal_role"))
			case errors.Is(authErr.Err, authService.ErrTenantNotFound):
				// School deactivated or deleted between challenge and
				// exchange — same 404 the other school surfaces return.
				common.RenderError(w, r, common.ErrorNotFound(authService.ErrTenantNotFound))
			case errors.Is(authErr.Err, authService.ErrTenantAccessDenied):
				// Membership revoked between challenge and exchange. Without
				// this case the mint guard's refusal fell through to a 500,
				// which reads as "our fault, retry" instead of "you are no
				// longer at this school".
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTenantAccessDenied))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	if rememberDevice {
		if err := rs.issueTrustedDeviceCookie(w, r, accountID, tenantID); err != nil {
			// Don't fail the whole login — log and proceed.
			slog.Default().Warn("failed to issue trusted-device cookie",
				slog.Int64("account_id", accountID),
				slog.String("error", err.Error()),
			)
		}
	}

	render.JSON(w, r, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// mfaResend re-issues an email code against the existing challenge token.
// Rate-limited inside the service; returns the renewed challenge JWT. Scope
// is enforced like on verify: a tenant- or operator-portal challenge cannot
// be driven (and its resend budget burned) through the school endpoint.
func (rs *Resource) mfaResend(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	req := &common.MFAResendRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	renewed, err := rs.MFAService.ResendChallengeForScope(r.Context(), req.ChallengeToken, common.ParseClientIP(r), jwt.MFAChallengeScopeSchool)
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	render.JSON(w, r, common.MFAResendResponse{ChallengeToken: renewed})
}

// EnrollStartResponse carries the challenge token minted alongside the
// enrollment email code. The client must send it back at
// /school/auth/mfa/enroll/confirm — see mfaEnrollConfirm for why the code
// alone is not a safe key.
type EnrollStartResponse struct {
	ChallengeToken string `json:"challenge_token"`
}

// mfaEnrollStart triggers the enrollment email code for a school-scope
// enrollment token (minted by the school login for accounts on an
// MFA-required school without a credential). Mirrors the tenant handler's
// defense-in-depth: the shared MFAEnrollmentAuthenticator accepts every
// enrollment scope, so this handler must pin the SCHOOL scope — a tenant-
// or platform-scope enrollment token is refused even when account_id
// matches.
//
// Unlike the tenant endpoint's 204, this returns the challenge token the
// service just minted; the confirm step is bound to that exact challenge.
func (rs *Resource) mfaEnrollStart(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims, ok := jwt.EnrollmentClaimsFromCtx(r.Context())
	if !ok || claims.AccountID == 0 || claims.Scope != jwt.MFAEnrollmentScopeSchool || claims.TenantID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	challengeToken, err := rs.MFAService.StartChallenge(r.Context(), claims.AccountID, claims.TenantID, jwt.MFAChallengeScopeSchool, common.ParseClientIP(r))
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	render.JSON(w, r, EnrollStartResponse{ChallengeToken: challengeToken})
}

// mfaEnrollConfirm verifies the emailed code, marks the account as enrolled,
// and mints a SCHOOL-scope session — the school sibling of the tenant
// enroll-confirm, so the enrollment detour never converts a school login
// into a tenant session.
//
// The confirm is bound to the exact CHALLENGE the enroll/start step minted:
// VerifyChallengeForOwner resolves the challenge row named in the token and
// refuses any scope but school. Verifying "the account's newest active code"
// instead — what VerifyCodeForAccount does — would let a concurrent challenge
// from another portal, or from another school, be consumed here: the same
// person logging into the tenant portal in a second tab is enough to produce
// one.
//
// It is bound to the enrollment token's IDENTITY as well, and that check runs
// inside the service, before the code is compared and thus before the
// single-use consume. The account/school comparison used to happen out here on
// the returned VerifiedChallenge, which was one step too late: a valid foreign
// challenge was already marked consumed by the time this handler rejected it,
// so an attacker holding someone else's school challenge token could kill that
// person's login without ever getting a session out of it.
func (rs *Resource) mfaEnrollConfirm(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFA(w, r) {
		return
	}
	claims, ok := jwt.EnrollmentClaimsFromCtx(r.Context())
	// Same defense-in-depth as mfaEnrollStart — school endpoint rejects
	// tenant- and platform-scope enrollment tokens even when account_id
	// matches.
	if !ok || claims.AccountID == 0 || claims.Scope != jwt.MFAEnrollmentScopeSchool || claims.TenantID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	// Same body shape as the verify endpoint: challenge token + code (+
	// remember_device), because this IS a challenge redemption.
	req := &common.MFAVerifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// The enroll routes run under MFAEnrollmentAuthenticator, NOT the
	// tenant transaction middleware, so tenant.FromContext is 0 here.
	// Inject the tenant from the enrollment claims so the MFA audit
	// events land in audit.auth_events instead of being dropped (mirrors
	// the tenant enroll-confirm).
	ctx := tenant.WithTenantID(r.Context(), claims.TenantID)

	verified, err := rs.MFAService.VerifyChallengeForOwner(
		ctx, req.ChallengeToken, req.Code, jwt.MFAChallengeScopeSchool, claims.AccountID, claims.TenantID,
	)
	if err != nil {
		mapMFAError(w, r, err)
		return
	}
	// The service already refused any challenge that is not this account's at
	// this school; re-asserting it here costs nothing and keeps the handler
	// honest if the binding ever moves.
	if verified == nil || verified.AccountID != claims.AccountID || verified.TenantID != claims.TenantID {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}

	if err := rs.MFAService.Enroll(ctx, claims.AccountID); err != nil {
		// Already enrolled is fine — a retried request must still produce
		// a valid session.
		if !errors.Is(err, authService.ErrMFAAlreadyEnrolled) {
			mapMFAError(w, r, err)
			return
		}
	}

	rs.completeSchoolExchange(w, r, claims.AccountID, claims.TenantID, req.RememberDevice)
}

// switchSchool handles POST /school/auth/switch-school. The account id comes
// from the school-scope claims; the target school must be an active mapping
// where the account holds a school-portal role.
func (rs *Resource) switchSchool(w http.ResponseWriter, r *http.Request) {
	req := &SwitchSchoolRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())

	accessToken, refreshToken, err := rs.AuthService.SwitchSchool(
		r.Context(), int64(claims.ID), req.TenantSlug,
		common.GetClientIPString(r), r.Header.Get(headerUserAgent),
	)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountNotFound))
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountInactive))
			case errors.Is(authErr.Err, authService.ErrTenantNotFound):
				common.RenderError(w, r, common.ErrorNotFound(authService.ErrTenantNotFound))
			case errors.Is(authErr.Err, authService.ErrTenantAccessDenied):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTenantAccessDenied))
			case errors.Is(authErr.Err, authService.ErrAccountNoSchoolPortalRole):
				common.RenderError(w, r, common.ErrorForbiddenWithCode(
					authService.ErrAccountNoSchoolPortalRole, "no_school_portal_role"))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	render.JSON(w, r, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// requireMFA returns true when the MFA service is wired in and writes a 503
// response otherwise. Use as the first line of every MFA handler.
func (rs *Resource) requireMFA(w http.ResponseWriter, r *http.Request) bool {
	return common.RequireDependency(w, r, rs.MFAService != nil, errMFAServiceUnavailable)
}

// mapMFAError translates known MFA-service errors into HTTP responses —
// same mapping as the tenant portal's MFA surface.
func mapMFAError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, authService.ErrMFAChallengeTokenInvalid),
		errors.Is(err, authService.ErrMFACodeInvalid),
		errors.Is(err, authService.ErrMFAUnsupportedScope):
		common.RenderError(w, r, common.ErrorUnauthorized(err))
	case errors.Is(err, authService.ErrMFALocked),
		errors.Is(err, authService.ErrMFARateLimited):
		common.RenderError(w, r, common.ErrorTooManyRequests(err))
	case errors.Is(err, authService.ErrMFAStatusUnavailable):
		// Fail-closed status/rate-limit lookup — the same 503 the school
		// login returns for it, so resend and enroll/start don't answer a
		// transient database problem with a 500 the client won't retry.
		common.RenderError(w, r, common.ErrorServiceUnavailable(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// issueTrustedDeviceCookie mirrors the tenant portal's cookie issuance.
// When the tenant has disabled trusted devices, the service returns an
// empty cookie value and no Set-Cookie header is written.
func (rs *Resource) issueTrustedDeviceCookie(w http.ResponseWriter, r *http.Request, accountID, tenantID int64) error {
	cookieValue, expiresAt, err := rs.MFAService.IssueTrustedDevice(
		r.Context(), accountID, tenantID, r.Header.Get(headerUserAgent), common.ParseClientIP(r),
	)
	if err != nil {
		return err
	}
	if cookieValue == "" {
		return nil
	}
	http.SetCookie(w, &http.Cookie{
		Name:     trustedDeviceCookieName,
		Value:    cookieValue,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}
