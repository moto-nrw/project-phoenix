package catererportalhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// PasswordResetRuntime is the caterer portal's password reset, bound by the
// root to Identity & Access: the link reaches caterer accounts only and lands
// on the caterer host.
type PasswordResetRuntime struct {
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

func (runtime PasswordResetRuntime) complete() bool {
	return runtime.Initiate != nil && runtime.Reset != nil && runtime.LinkUnusable != nil && runtime.TooWeak != nil && runtime.RetryAfter != nil
}

var (
	errResetUnavailable  = errors.New("password reset is not available")
	errResetRateLimited  = errors.New("too many password reset requests")
	errResetLinkUnusable = errors.New("invalid or expired reset token")
	errResetTooWeak      = errors.New("password does not meet the requirements")
	errResetMismatch     = errors.New("passwords do not match")
)

type resetRequest struct {
	Email string `json:"email"`
}

type resetConfirmRequest struct {
	Token           string `json:"token"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// initiatePasswordReset mails a reset link when the address belongs to a
// caterer. The answer is the same for every address.
func (rs *Resource) initiatePasswordReset(w http.ResponseWriter, r *http.Request) {
	var request resetRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || strings.TrimSpace(request.Email) == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("email is required")))
		return
	}
	if !rs.runtime.Resets.complete() {
		common.RenderError(w, r, common.ErrorInternalServer(errResetUnavailable))
		return
	}
	if err := rs.runtime.Resets.Initiate(r.Context(), strings.ToLower(strings.TrimSpace(request.Email))); err != nil {
		if seconds, limited := rs.runtime.Resets.RetryAfter(err); limited {
			if seconds > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(seconds))
			}
			common.RenderError(w, r, common.ErrorTooManyRequestsWithCode(errResetRateLimited, common.CodeIdentityPasswordResetRateLimited))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "If the email exists, a password reset link has been sent")
}

// resetPassword sets a new password through a reset link.
func (rs *Resource) resetPassword(w http.ResponseWriter, r *http.Request) {
	var request resetConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Token == "" || request.NewPassword == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("token and new_password are required")))
		return
	}
	if request.NewPassword != request.ConfirmPassword {
		common.RenderError(w, r, common.ErrorInvalidOnField(errResetMismatch, common.CodeIdentityPasswordMismatch, "confirm_password"))
		return
	}
	if !rs.runtime.Resets.complete() {
		common.RenderError(w, r, common.ErrorInternalServer(errResetUnavailable))
		return
	}
	if err := rs.runtime.Resets.Reset(r.Context(), request.Token, request.NewPassword); err != nil {
		switch {
		case rs.runtime.Resets.LinkUnusable(err):
			common.RenderError(w, r, common.ErrorGoneWithCode(errResetLinkUnusable, common.CodeIdentityPasswordResetLinkInvalid))
		case rs.runtime.Resets.TooWeak(err):
			common.RenderError(w, r, common.ErrorInvalidOnField(errResetTooWeak, common.CodeIdentityPasswordTooWeak, "new_password"))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Password reset successfully")
}
