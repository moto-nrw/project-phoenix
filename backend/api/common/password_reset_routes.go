package common

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
)

// PasswordResetRoutes are the two public reset routes every account portal
// mounts: POST …/auth/password-reset and …/confirm. The root binds the funcs
// to Identity & Access with the portal's reset scope, so the link reaches
// only that portal's accounts and lands on its host, and one malformed
// request gets one answer in every portal.
type PasswordResetRoutes struct {
	// Initiate answers nil whether or not a link was issued, so the
	// response cannot be used to enumerate addresses.
	Initiate func(ctx context.Context, email string) error
	Reset    func(ctx context.Context, token, newPassword string) error
	// LinkUnusable reports a link that is unknown, expired or spent.
	LinkUnusable func(error) bool
	// TooWeak reports a password the policy refused.
	TooWeak func(error) bool
	// RetryAfter reports the rate limit and the seconds until the address
	// may request again.
	RetryAfter func(error) (int, bool)
}

// Complete reports whether every reset function is bound.
func (routes PasswordResetRoutes) Complete() bool {
	return routes.Initiate != nil && routes.Reset != nil && routes.LinkUnusable != nil && routes.TooWeak != nil && routes.RetryAfter != nil
}

var (
	errResetUnavailable  = errors.New("password reset is not available")
	errResetRateLimited  = errors.New("too many password reset requests")
	errResetLinkUnusable = errors.New("invalid or expired reset token")
	errResetTooWeak      = errors.New("password does not meet the requirements")
	errResetMismatch     = errors.New("passwords do not match")
)

type passwordResetRequest struct {
	Email string `json:"email"`
}

type passwordResetConfirmRequest struct {
	Token           string `json:"token"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// ServeInitiate mails a reset link when the address belongs to an account of
// the portal. The answer is the same for every well-formed address; a
// rate-limited one gets 429 with the Retry-After the frontend countdown
// reads.
func (routes PasswordResetRoutes) ServeInitiate(w http.ResponseWriter, r *http.Request) {
	var request passwordResetRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	email := strings.ToLower(strings.TrimSpace(request.Email))
	if err != nil || validation.Validate(email, validation.Required, is.Email) != nil {
		RenderError(w, r, ErrorInvalidRequest(errors.New("a valid email is required")))
		return
	}
	if !routes.Complete() {
		RenderError(w, r, ErrorInternalServer(errResetUnavailable))
		return
	}
	if err := routes.Initiate(r.Context(), email); err != nil {
		if seconds, limited := routes.RetryAfter(err); limited {
			if seconds > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(seconds))
			}
			RenderError(w, r, ErrorTooManyRequestsWithCode(errResetRateLimited, CodeIdentityPasswordResetRateLimited))
			return
		}
		RenderError(w, r, ErrorInternalServer(err))
		return
	}
	Respond(w, r, http.StatusOK, nil, "If the email exists, a password reset link has been sent")
}

// ServeConfirm sets a new password through a reset link.
func (routes PasswordResetRoutes) ServeConfirm(w http.ResponseWriter, r *http.Request) {
	var request passwordResetConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Token == "" || request.NewPassword == "" {
		RenderError(w, r, ErrorInvalidRequest(errors.New("token and new_password are required")))
		return
	}
	if request.NewPassword != request.ConfirmPassword {
		RenderError(w, r, ErrorInvalidOnField(errResetMismatch, CodeIdentityPasswordMismatch, "confirm_password"))
		return
	}
	if !routes.Complete() {
		RenderError(w, r, ErrorInternalServer(errResetUnavailable))
		return
	}
	if err := routes.Reset(r.Context(), request.Token, request.NewPassword); err != nil {
		switch {
		case routes.LinkUnusable(err):
			RenderError(w, r, ErrorGoneWithCode(errResetLinkUnusable, CodeIdentityPasswordResetLinkInvalid))
		case routes.TooWeak(err):
			RenderError(w, r, ErrorInvalidOnField(errResetTooWeak, CodeIdentityPasswordTooWeak, "new_password"))
		default:
			RenderError(w, r, ErrorInternalServer(err))
		}
		return
	}
	Respond(w, r, http.StatusOK, nil, "Password reset successfully")
}
