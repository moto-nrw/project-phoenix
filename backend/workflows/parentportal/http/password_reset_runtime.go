package parent

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// Password reset is owned by Identity & Access (#2722). The composition root
// binds its reset routes with the parent scope, so the link reaches only
// guardians and lands on the parents host. The parents portal keeps its own
// handlers: they validate the address and the password length before the
// owner is asked.

// PasswordResetRuntime is the parents portal's reset port, the shape of the
// shared reset routes. Initiate answers nil whether or not a link was issued.
type PasswordResetRuntime struct {
	Initiate     func(ctx context.Context, email string) error
	Reset        func(ctx context.Context, token, newPassword string) error
	LinkUnusable func(error) bool
	TooWeak      func(error) bool
	RetryAfter   func(error) (int, bool)
}

func (rt PasswordResetRuntime) complete() bool {
	return rt.Initiate != nil && rt.Reset != nil && rt.LinkUnusable != nil && rt.TooWeak != nil && rt.RetryAfter != nil
}

var (
	// ErrPasswordResetUnavailable reports a resource composed without the
	// reset routes.
	ErrPasswordResetUnavailable = errors.New("password reset is not composed")
	// ErrPasswordResetRateLimited is the 429 body of a rate-limited request.
	ErrPasswordResetRateLimited = errors.New("too many password reset requests")
	// ErrPasswordTooWeak is the 400 body of a refused credential.
	ErrPasswordTooWeak = errors.New("password doesn't meet complexity requirements")
)

// renderPasswordResetRateLimit answers a rate-limited reset request with 429
// and the Retry-After header the frontend countdown reads, and reports
// whether it handled the error.
func (rs *Resource) renderPasswordResetRateLimit(w http.ResponseWriter, r *http.Request, err error) bool {
	seconds, limited := rs.Resets.RetryAfter(err)
	if !limited {
		return false
	}
	if seconds > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
	}
	common.RenderError(w, r, common.ErrorTooManyRequestsWithCode(ErrPasswordResetRateLimited, common.CodeIdentityPasswordResetRateLimited))
	return true
}
