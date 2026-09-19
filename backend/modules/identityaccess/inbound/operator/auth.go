package operator

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/rotation"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	jwtPkg "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// TrustedDeviceCookieName mirrors the tenant-side constant. The operator
// subdomain has its own cookie scope, so the shared name cannot collide
// with tenant browsers.
const TrustedDeviceCookieName = "mfa_trust_device"

// LoginStatusAuthenticated is the status of a login response that carries a
// full token pair.
const LoginStatusAuthenticated = string(identityaccess.LoginStatusAuthenticated)

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Bind validates the login request
func (req *LoginRequest) Bind(_ *http.Request) error {
	return nil
}

// LoginResponse represents the login response. The shape is discriminated
// by `status`:
//   - status == "authenticated" → access_token + refresh_token + operator
//   - status == "mfa_required"  → challenge_token + masked_email
//   - status == "mfa_enrollment_required" → enrollment access_token + operator
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

// RefreshTokenResponse represents the refresh token response
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func operatorResponse(operator *identityaccess.Operator) *OperatorResponse {
	if operator == nil {
		return nil
	}
	return &OperatorResponse{ID: operator.ID, Email: operator.Email, DisplayName: operator.DisplayName}
}

// Login handles operator login behind the mandatory MFA gate. It forwards
// the trusted-device cookie and maps the discriminated result onto the
// login response.
func (rs *Resource) Login(w http.ResponseWriter, r *http.Request) {
	req := &LoginRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, rs.responses.InvalidRequest(err))
		return
	}
	if req.Email == "" || req.Password == "" {
		common.RenderError(w, r, rs.responses.InvalidCredentials())
		return
	}

	ipString := ""
	if clientIP := common.ParseClientIP(r); clientIP != nil {
		ipString = clientIP.String()
	}
	var trustedDeviceCookie string
	if c, err := r.Cookie(TrustedDeviceCookieName); err == nil {
		trustedDeviceCookie = c.Value
	}

	result, err := rs.identity.LoginOperatorWithMFAGate(
		r.Context(), req.Email, req.Password, ipString, r.Header.Get("User-Agent"), trustedDeviceCookie,
	)
	if err != nil {
		slog.Default().ErrorContext(r.Context(), "operator login error",
			slog.String("error", err.Error()))
		common.RenderError(w, r, rs.authError(err))
		return
	}

	switch result.Status {
	case identityaccess.LoginStatusMFARequired:
		tde := result.TrustedDeviceEnabled
		tdd := result.TrustedDeviceDays
		common.Respond(w, r, http.StatusOK, &LoginResponse{
			Status:               string(identityaccess.LoginStatusMFARequired),
			ChallengeToken:       result.ChallengeToken,
			MaskedEmail:          result.MaskedEmail,
			TrustedDeviceEnabled: &tde,
			TrustedDeviceDays:    &tdd,
		}, "MFA verification required")
	case identityaccess.LoginStatusMFAEnrollmentRequired:
		common.Respond(w, r, http.StatusOK, &LoginResponse{
			Status:                string(identityaccess.LoginStatusMFAEnrollmentRequired),
			AccessToken:           result.AccessToken,
			MaskedEmail:           result.MaskedEmail,
			MFAEnrollmentRequired: true,
			Operator:              operatorResponse(result.Operator),
		}, "MFA enrollment required")
	default:
		common.Respond(w, r, http.StatusOK, &LoginResponse{
			Status:       LoginStatusAuthenticated,
			AccessToken:  result.AccessToken,
			RefreshToken: result.RefreshToken,
			Operator:     operatorResponse(result.Operator),
		}, "Login successful")
	}
}

// RefreshToken handles operator token refresh. The refresh JWT was verified
// by the router's middleware; only operator-scoped tokens that name a
// persisted session reach the capability.
func (rs *Resource) RefreshToken(w http.ResponseWriter, r *http.Request) {
	if jwtPkg.RefreshTokenFromCtx(r.Context()) == "" {
		common.RenderError(w, r, rs.responses.Unauthorized())
		return
	}

	var claims jwtPkg.RefreshClaims
	_, rawClaims, _ := jwtauth.FromContext(r.Context())
	if err := claims.ParseClaims(rawClaims); err != nil {
		common.RenderError(w, r, rs.responses.Unauthorized())
		return
	}
	// Pre-fix deterministic operator refresh tokens had no persisted session
	// and no platform scope claim; reject them before the lookup, as well as
	// every tenant or user token.
	if claims.Scope != "platform" || claims.Token == "" {
		common.RenderError(w, r, rs.responses.Unauthorized())
		return
	}

	ctx := rotation.WithRecoveryProof(r.Context(), r.Header.Get(rotation.RecoveryProofHeader))
	accessToken, refreshToken, err := rs.identity.RefreshOperatorToken(ctx, int64(claims.ID), claims.Token)
	if err != nil {
		common.RenderError(w, r, rs.authError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, &RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, "Token refreshed")
}
