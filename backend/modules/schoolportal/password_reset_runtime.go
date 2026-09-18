package schoolportal

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// Password reset is owned by Identity & Access (#2722). The school portal
// consumes it through the runtime below, which the composition root binds to
// the owner's capability — the same shape the People Directory guardian
// routes use. The portal names no owner contract, so the two error texts and
// the Retry-After header stay this package's business.

// PasswordResetRuntime is the reset port of the school portal. Initiate
// answers nil whether or not a link was issued, so the response cannot be
// used to enumerate addresses; the classifiers say what a refused Reset was.
type PasswordResetRuntime struct {
	// Initiate issues a reset link for an account that holds a school-portal
	// role, so the link lands on the school host.
	Initiate func(ctx context.Context, email string) error
	Reset    func(ctx context.Context, token, newPassword string) error
	// LinkUnusable reports a link that is unknown, expired or already spent.
	// The response never says which of the three it was.
	LinkUnusable func(error) bool
	// TooWeak reports a credential the password policy refused.
	TooWeak func(error) bool
	// RetryAfter reports the rate-limit rejection and the seconds until the
	// address may request again; zero seconds means the window already
	// elapsed.
	RetryAfter func(error) (int, bool)
}

func (rt *PasswordResetRuntime) complete() bool {
	return rt != nil && rt.Initiate != nil && rt.Reset != nil &&
		rt.LinkUnusable != nil && rt.TooWeak != nil && rt.RetryAfter != nil
}

// ErrPasswordResetUnavailable reports a resource composed without the reset
// runtime; the routes answer 500, as a service composed without the reset
// dependencies did.
var ErrPasswordResetUnavailable = errors.New("password reset is not composed")

// ErrPasswordResetRateLimited is the 429 body of a rate-limited request.
var ErrPasswordResetRateLimited = errors.New("too many password reset requests")

// ErrPasswordTooWeak is the 400 body of a refused credential.
var ErrPasswordTooWeak = errors.New("password doesn't meet complexity requirements")

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
	common.RenderError(w, r, common.ErrorTooManyRequests(ErrPasswordResetRateLimited))
	return true
}
