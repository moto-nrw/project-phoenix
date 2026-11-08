package account

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

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
	common.RenderError(w, r, common.ErrorTooManyRequestsWithCode(identityaccess.ErrPasswordResetRateLimited, common.CodeIdentityPasswordResetRateLimited))
	return true
}
