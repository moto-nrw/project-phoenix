package parent

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-nrw/project-phoenix/api/common"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// LoginRequest is the body shape for POST /parent/auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Bind normalizes + validates the request.
func (req *LoginRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.Password, validation.Required),
	)
}

// TokenResponse is the success response shape — same as the tenant
// login response so the frontend NextAuth provider can consume both.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// PasswordResetRequest is the body shape for POST /parent/auth/password-reset.
type PasswordResetRequest struct {
	Email string `json:"email"`
}

// Bind normalizes + validates the password reset request.
func (req *PasswordResetRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
	)
}

// PasswordResetConfirmRequest is the body shape for POST /parent/auth/password-reset/confirm.
type PasswordResetConfirmRequest struct {
	Token           string `json:"token"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// Bind validates the password reset confirmation request.
func (req *PasswordResetConfirmRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Token, validation.Required),
		validation.Field(&req.NewPassword, validation.Required, validation.Length(8, 0)),
		validation.Field(&req.ConfirmPassword, validation.Required, validation.By(func(_ interface{}) error {
			if req.NewPassword != req.ConfirmPassword {
				return errors.New("passwords do not match")
			}
			return nil
		})),
	)
}

// login authenticates a guardian via email/password and issues a
// parent-scope JWT. Refuses accounts that don't have a guardian role
// at any tenant.
func (rs *Resource) login(w http.ResponseWriter, r *http.Request) {
	req := &LoginRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	ipAddress := getClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	accessToken, refreshToken, err := rs.AuthService.LoginParentWithAudit(
		r.Context(), req.Email, req.Password, ipAddress, userAgent,
	)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidCredentials),
				errors.Is(authErr.Err, authService.ErrAccountNotFound):
				// Mask the specific cause to prevent account
				// enumeration. Same pattern as the tenant login.
				common.RenderError(w, r, common.ErrorUnauthorizedWithCode(
					authService.ErrInvalidCredentials, "invalid_credentials"))
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				// Distinct code so the frontend can show
				// "your account is disabled, contact the school"
				// instead of a generic credentials error.
				common.RenderError(w, r, common.ErrorUnauthorizedWithCode(
					authService.ErrAccountInactive, "account_inactive"))
			case errors.Is(authErr.Err, authService.ErrAccountNoGuardianRole):
				// 403 with a stable code — frontend masks this as
				// invalid_credentials in the user-facing copy (the
				// German copy already includes a staff-login hint
				// for this case) to avoid leaking that the email is
				// a known staff account.
				common.RenderError(w, r, common.ErrorForbiddenWithCode(
					authService.ErrAccountNoGuardianRole, "not_a_guardian"))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, "Login successful")
}

// initiatePasswordReset sends a parent-portal password reset link when the
// email belongs to an account with guardian access. It always returns the same
// success body for unknown or non-guardian emails to avoid account enumeration.
func (rs *Resource) initiatePasswordReset(w http.ResponseWriter, r *http.Request) {
	req := &PasswordResetRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if _, err := rs.AuthService.InitiateParentPasswordReset(r.Context(), req.Email); err != nil {
		var rateErr *authService.RateLimitError
		if errors.As(err, &rateErr) {
			retryAfterSeconds := rateErr.RetryAfterSeconds(time.Now())
			if retryAfterSeconds > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
			} else if !rateErr.RetryAt.IsZero() {
				w.Header().Set("Retry-After", rateErr.RetryAt.UTC().Format(http.TimeFormat))
			}

			common.RenderError(w, r, common.ErrorTooManyRequests(authService.ErrRateLimitExceeded))
			return
		}

		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "If the email exists, a password reset link has been sent")
}

// resetPassword confirms a parent password reset token. Tokens are shared with
// the staff reset flow; this route exists so parent pages never need to call
// the tenant auth API surface.
func (rs *Resource) resetPassword(w http.ResponseWriter, r *http.Request) {
	req := &PasswordResetConfirmRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if err := rs.AuthService.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidToken),
				errors.Is(authErr.Err, sql.ErrNoRows):
				// 410 Gone: the token is invalid, already used, or expired.
				// The parents reset page maps this status to the "request a
				// new link" copy. Distinct from the 400 weak-password case
				// below, which tells the user to fix the password itself —
				// collapsing both into 400 sent the wrong remedy.
				common.RenderError(w, r, common.ErrorGone(errors.New("invalid or expired reset token")))
				return
			case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
				common.RenderError(w, r, common.ErrorInvalidRequest(authService.ErrPasswordTooWeak))
				return
			}
		}

		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Password reset successfully")
}

// getClientIP returns the router-selected client IP for audit/rate-limit flows.
func getClientIP(r *http.Request) string {
	return common.GetClientIPString(r)
}
