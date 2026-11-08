package operator

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/portalauth"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Bind validates the login request
func (req *LoginRequest) Bind(_ *http.Request) error {
	return nil
}

// LoginResponse is the account portals' login answer with the operator on
// the authenticated and the enrollment branches.
type LoginResponse struct {
	portalauth.LoginResponse
	Operator *OperatorResponse `json:"operator,omitempty"`
}

// OperatorResponse represents an operator in the response
type OperatorResponse struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

func operatorResponse(operator *identityaccess.Operator) *OperatorResponse {
	if operator == nil {
		return nil
	}
	return &OperatorResponse{ID: operator.ID, Email: operator.Email, DisplayName: operator.DisplayName}
}

// Login signs an operator in behind the mandatory MFA gate. The
// trusted-device cookie lets the gate skip the code on a remembered device.
func (rs *Resource) Login(w http.ResponseWriter, r *http.Request) {
	req := &LoginRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	if req.Email == "" || req.Password == "" {
		common.RenderError(w, r, common.OperatorInvalidCredentials())
		return
	}
	ipAddress := ""
	if clientIP := common.ParseClientIP(r); clientIP != nil {
		ipAddress = clientIP.String()
	}
	result, err := rs.identity.LoginOperatorWithMFAGate(
		r.Context(), req.Email, req.Password, ipAddress, r.UserAgent(), common.TrustedDevice(r),
	)
	if err != nil {
		slog.Default().ErrorContext(r.Context(), "operator login error",
			slog.String("error", err.Error()))
		common.RenderError(w, r, AuthErrorRenderer(err))
		return
	}
	body := LoginResponse{LoginResponse: portalauth.LoginBody(identityaccess.LoginResult{
		Status: result.Status, AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
		ChallengeToken: result.ChallengeToken, MaskedEmail: result.MaskedEmail,
		TrustedDeviceEnabled: result.TrustedDeviceEnabled, TrustedDeviceDays: result.TrustedDeviceDays,
	})}
	message := "Login successful"
	switch result.Status {
	case identityaccess.LoginStatusMFARequired:
		message = "MFA verification required"
	case identityaccess.LoginStatusMFAEnrollmentRequired:
		message = "MFA enrollment required"
		body.Operator = operatorResponse(result.Operator)
	default:
		body.Operator = operatorResponse(result.Operator)
	}
	common.Respond(w, r, http.StatusOK, &body, message)
}

// RefreshToken handles operator token refresh. The router's middleware
// verified the refresh JWT; the capability refuses every token that does not
// name a persisted operator session.
func (rs *Resource) RefreshToken(w http.ResponseWriter, r *http.Request) {
	presented := sessiontoken.RefreshTokenFromCtx(r.Context())
	if presented == "" {
		common.RenderError(w, r, common.OperatorUnauthorized())
		return
	}

	accessToken, refreshToken, err := rs.identity.RefreshOperatorToken(r.Context(), presented, r.Header.Get(sessiontoken.RecoveryProofHeader))
	if err != nil {
		common.RenderError(w, r, AuthErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, &portalauth.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, "Token refreshed")
}

// Logout revokes the operator session family of the logged-in access token
// (#5194). The route runs behind RequiresActiveOperator, which proved that
// family live and the operator's own. 204 confirms the revocation; a failed
// revocation answers 503 so the browser does not take the logout for done.
func (rs *Resource) Logout(w http.ResponseWriter, r *http.Request) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	err := rs.identity.LogoutOperator(r.Context(), int64(claims.ID), claims.FamilyID)
	switch {
	case err == nil:
		common.RespondNoContent(w, r)
	case errors.Is(err, identityaccess.ErrOperatorRefreshTokenInvalid):
		common.RenderError(w, r, common.OperatorUnauthorized())
	default:
		slog.ErrorContext(r.Context(), "operator logout did not revoke the session family",
			slog.Int("operator_id", claims.ID),
			slog.String("error", err.Error()),
		)
		common.RenderError(w, r, common.OperatorServiceUnavailable("Logout could not revoke the session, please retry"))
	}
}
