package account

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The staff reset routes classify the owner's outcomes here so both
// handlers read the same rules. The frontend countdown reads the
// Retry-After header the rate limit sets, so its shape is part of the
// contract.

// renderPasswordResetRateLimit answers a rate-limited reset request with 429
// and the Retry-After header, and reports whether it handled the error.
func renderPasswordResetRateLimit(w http.ResponseWriter, r *http.Request, err error) bool {
	var limited *identityaccess.PasswordResetRateLimitError
	if !errors.As(err, &limited) {
		return false
	}
	// Prefer Retry-After seconds, fallback to RFC1123 format
	if seconds := limited.RetryAfterSeconds(time.Now()); seconds > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
	} else if !limited.RetryAt.IsZero() {
		w.Header().Set("Retry-After", limited.RetryAt.UTC().Format(http.TimeFormat))
	}
	common.RenderError(w, r, common.ErrorTooManyRequests(identityaccess.ErrPasswordResetRateLimited))
	return true
}

// passwordResetLinkUnusable reports a reset confirmation whose link cannot
// be spent: unknown, expired, already used, or a missing row. The response
// never says which of those it was.
func passwordResetLinkUnusable(err error) bool {
	err = passwordResetCause(err)
	return errors.Is(err, identityaccess.ErrInvalidToken) || errors.Is(err, identityaccess.ErrRecordMissing)
}

// passwordTooWeak reports a credential the password policy refused.
func passwordTooWeak(err error) bool {
	return errors.Is(passwordResetCause(err), identityaccess.ErrPasswordTooWeak)
}

// passwordResetCause unwraps the operation envelope the module reports so
// the classification reads the cause, not the wrapper.
func passwordResetCause(err error) error {
	var operation *identityaccess.AuthenticationError
	if errors.As(err, &operation) && operation.Err != nil {
		return operation.Err
	}
	return err
}
