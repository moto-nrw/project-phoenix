package auth

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// ChangePasswordRequest represents the change password request payload
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// Bind validates the change password request
func (req *ChangePasswordRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.CurrentPassword, validation.Required),
		validation.Field(&req.NewPassword, validation.Required, validation.Length(8, 0)),
		validation.Field(&req.ConfirmPassword, validation.Required, validation.By(func(value interface{}) error {
			if req.NewPassword != req.ConfirmPassword {
				return errors.New(errPasswordsNotMatch)
			}
			return nil
		})),
	)
}

// changePassword handles password change
func (rs *Resource) changePassword(w http.ResponseWriter, r *http.Request) {
	req := &ChangePasswordRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get user ID from JWT claims
	claims := jwt.ClaimsFromCtx(r.Context())

	err := rs.AuthService.ChangePassword(r.Context(), claims.ID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidCredentials):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrInvalidCredentials))
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrAccountNotFound))
			case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
				common.RenderError(w, r, ErrorInvalidRequest(authService.ErrPasswordTooWeak))
			default:
				common.RenderError(w, r, ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// PasswordResetRequest represents the password reset request payload
type PasswordResetRequest struct {
	Email string `json:"email"`
}

// Bind validates the password reset request
func (req *PasswordResetRequest) Bind(_ *http.Request) error {
	req.Email = trimAndLowerEmail(req.Email)

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, validation.By(validateEmail)),
	)
}

// PasswordResetConfirmRequest represents the password reset confirm request payload
type PasswordResetConfirmRequest struct {
	Token           string `json:"token"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// Bind validates the password reset confirm request
func (req *PasswordResetConfirmRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Token, validation.Required),
		validation.Field(&req.NewPassword, validation.Required, validation.Length(8, 0)),
		validation.Field(&req.ConfirmPassword, validation.Required, validation.By(func(value interface{}) error {
			if req.NewPassword != req.ConfirmPassword {
				return errors.New(errPasswordsNotMatch)
			}
			return nil
		})),
	)
}

// initiatePasswordReset handles initiating a password reset
func (rs *Resource) initiatePasswordReset(w http.ResponseWriter, r *http.Request) {
	req := &PasswordResetRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Always return success to avoid revealing whether email exists, but handle rate limiting
	_, err := rs.AuthService.InitiatePasswordReset(r.Context(), req.Email)
	if err != nil {
		var rateErr *authService.RateLimitError
		if errors.As(err, &rateErr) {
			// Prefer Retry-After seconds, fallback to RFC1123 format
			retryAfterSeconds := rateErr.RetryAfterSeconds(time.Now())
			if retryAfterSeconds > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
			} else if !rateErr.RetryAt.IsZero() {
				w.Header().Set("Retry-After", rateErr.RetryAt.UTC().Format(http.TimeFormat))
			}

			common.RenderError(w, r, common.ErrorTooManyRequests(authService.ErrRateLimitExceeded))
			return
		}

		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	log.Printf("Password reset initiated for email=%s", req.Email)

	common.Respond(w, r, http.StatusOK, nil, "If the email exists, a password reset link has been sent")
}

// resetPassword handles resetting password with token
func (rs *Resource) resetPassword(w http.ResponseWriter, r *http.Request) {
	req := &PasswordResetConfirmRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	if err := rs.AuthService.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		log.Printf("Password reset failed reason=%v", err)

		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidToken),
				errors.Is(authErr.Err, sql.ErrNoRows):
				// Both cases indicate the token is invalid or not found
				common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid or expired reset token")))
				return
			case errors.Is(authErr.Err, authService.ErrPasswordTooWeak):
				common.RenderError(w, r, ErrorInvalidRequest(authService.ErrPasswordTooWeak))
				return
			}
		}

		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	log.Printf("Password reset completed successfully")

	common.Respond(w, r, http.StatusOK, nil, "Password reset successfully")
}
