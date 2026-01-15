package auth

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

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

// cleanupExpiredTokens handles cleanup of expired tokens
func (rs *Resource) cleanupExpiredTokens(w http.ResponseWriter, r *http.Request) {
	count, err := rs.AuthService.CleanupExpiredTokens(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	response := map[string]int{"cleaned_tokens": count}
	common.Respond(w, r, http.StatusOK, response, "Expired tokens cleaned up successfully")
}
