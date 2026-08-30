package operator

import (
	"log/slog"
	"net"
	"net/http"

	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	jwtPkg "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/rotation"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// AuthResource handles operator authentication endpoints
type AuthResource struct {
	authService platformSvc.OperatorAuthService
}

// NewAuthResource creates a new auth resource
func NewAuthResource(authService platformSvc.OperatorAuthService) *AuthResource {
	return &AuthResource{
		authService: authService,
	}
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Bind validates the login request
func (req *LoginRequest) Bind(r *http.Request) error {
	return nil
}

// trustedDeviceCookieName mirrors the tenant-side constant. Same cookie name
// is fine — the operator subdomain has its own cookie scope, so there's no
// collision with tenant browsers.
const trustedDeviceCookieName = "mfa_trust_device"

// LoginResponse represents the login response. With issue #1308 MFA the
// shape is discriminated by `status`:
//   - status == "authenticated" → access_token + refresh_token + operator
//   - status == "mfa_required"  → challenge_token + masked_email
//
// mfa_enrollment_required is set on authenticated responses when the
// operator must enrol on next page (forced by hardcoded operator MFA
// policy).
type LoginResponse struct {
	Status                string            `json:"status"`
	AccessToken           string            `json:"access_token,omitempty"`
	RefreshToken          string            `json:"refresh_token,omitempty"`
	Operator              *OperatorResponse `json:"operator,omitempty"`
	ChallengeToken        string            `json:"challenge_token,omitempty"`
	MaskedEmail           string            `json:"masked_email,omitempty"`
	MFAEnrollmentRequired bool              `json:"mfa_enrollment_required,omitempty"`
	// TrustedDeviceEnabled is set on the mfa_required branch. Operator
	// MFA always exposes the trusted-device feature, but the field is
	// emitted for response-shape symmetry with the tenant login.
	TrustedDeviceEnabled *bool `json:"trusted_device_enabled,omitempty"`
	// TrustedDeviceDays mirrors the tenant response so the frontend can
	// render a dynamic "Auf diesem Gerät N Tage merken" label.
	TrustedDeviceDays *int `json:"trusted_device_days,omitempty"`
}

// OperatorResponse represents an operator in the response
type OperatorResponse struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// Login handles operator login. Now MFA-aware: it reads the
// mfa_trust_device cookie off the request and forwards it to
// LoginWithMFAGate, then maps the discriminated OperatorLoginResult to
// the shared LoginResponse JSON shape.
func (rs *AuthResource) Login(w http.ResponseWriter, r *http.Request) {
	req := &LoginRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	if req.Email == "" || req.Password == "" {
		common.RenderError(w, r, ErrInvalidCredentials())
		return
	}

	clientIP := getClientIP(r)
	ipString := ""
	if clientIP != nil {
		ipString = clientIP.String()
	}
	userAgent := r.Header.Get("User-Agent")

	var trustedDeviceCookie string
	if c, err := r.Cookie(trustedDeviceCookieName); err == nil {
		trustedDeviceCookie = c.Value
	}

	result, err := rs.authService.LoginWithMFAGate(
		r.Context(), req.Email, req.Password, ipString, userAgent, trustedDeviceCookie,
	)
	if err != nil {
		slog.Default().ErrorContext(r.Context(), "operator login error",
			slog.String("error", err.Error()))
		common.RenderError(w, r, AuthErrorRenderer(err))
		return
	}

	switch result.Status {
	case platformSvc.OperatorLoginStatusMFARequired:
		tde := result.TrustedDeviceEnabled
		tdd := result.TrustedDeviceDays
		common.Respond(w, r, http.StatusOK, &LoginResponse{
			Status:               string(platformSvc.OperatorLoginStatusMFARequired),
			ChallengeToken:       result.ChallengeToken,
			MaskedEmail:          result.MaskedEmail,
			TrustedDeviceEnabled: &tde,
			TrustedDeviceDays:    &tdd,
		}, "MFA verification required")
		return
	case platformSvc.OperatorLoginStatusMFAEnrollmentRequired:
		resp := &LoginResponse{
			Status:                string(platformSvc.OperatorLoginStatusMFAEnrollmentRequired),
			AccessToken:           result.AccessToken,
			MaskedEmail:           result.MaskedEmail,
			MFAEnrollmentRequired: true,
		}
		if result.Operator != nil {
			resp.Operator = &OperatorResponse{
				ID:          result.Operator.ID,
				Email:       result.Operator.Email,
				DisplayName: result.Operator.DisplayName,
			}
		}
		common.Respond(w, r, http.StatusOK, resp, "MFA enrollment required")
		return
	}

	resp := &LoginResponse{
		Status:       string(platformSvc.OperatorLoginStatusAuthenticated),
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	}
	if result.Operator != nil {
		resp.Operator = &OperatorResponse{
			ID:          result.Operator.ID,
			Email:       result.Operator.Email,
			DisplayName: result.Operator.DisplayName,
		}
	}
	common.Respond(w, r, http.StatusOK, resp, "Login successful")
}

// RefreshTokenResponse represents the refresh token response
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// RefreshToken handles operator token refresh
func (rs *AuthResource) RefreshToken(w http.ResponseWriter, r *http.Request) {
	// Extract refresh token string from context (set by AuthenticateRefreshJWT middleware)
	tokenStr := jwtPkg.RefreshTokenFromCtx(r.Context())
	if tokenStr == "" {
		common.RenderError(w, r, ErrUnauthorized())
		return
	}

	// Parse the refresh claims to get the operator ID
	var claims jwtPkg.RefreshClaims
	_, rawClaims, _ := jwtauth.FromContext(r.Context())
	if err := claims.ParseClaims(rawClaims); err != nil {
		common.RenderError(w, r, ErrUnauthorized())
		return
	}

	// Verify this is an operator-scoped refresh token, not a tenant/user token.
	// Pre-fix deterministic operator refresh tokens had no persisted session
	// and no platform scope claim; reject them here before the service lookup.
	if claims.Scope != "platform" || claims.Token == "" {
		common.RenderError(w, r, ErrUnauthorized())
		return
	}

	ctx := rotation.WithRecoveryProof(r.Context(), r.Header.Get(rotation.RecoveryProofHeader))
	accessToken, refreshToken, err := rs.authService.RefreshToken(ctx, int64(claims.ID), claims.Token)
	if err != nil {
		common.RenderError(w, r, AuthErrorRenderer(err))
		return
	}

	response := &RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}

	common.Respond(w, r, http.StatusOK, response, "Token refreshed")
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) net.IP {
	return common.ParseClientIP(r)
}
