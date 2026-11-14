package parent

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/moto-ogs/project-phoenix/api/common"
)

// mountSignIn mounts the owner's public sign-in routes. A sign-in proved the
// address: its holiday registrations join the account (ADR 0216). The
// workflow logs a failure; the sign-in stands.
func (rs *Resource) mountSignIn(r chi.Router) {
	if rs.Auth.Routes == nil {
		r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
			common.RequireDependency(w, r, false, errLoginUnavailable)
		})
		return
	}
	rs.Auth.Routes.MountSignIn(r, rs.claimHolidayRegistrations)
}

func (rs *Resource) claimHolidayRegistrations(ctx context.Context, email string) {
	if rs.ParentService != nil && email != "" {
		rs.ParentService.ClaimHolidayRegistrationsAfterSignIn(ctx, email)
	}
}

// mountSecondFactor mounts the session's routes of its second factor
// (#7051); the credential checks sit behind the auth rate limit.
func (rs *Resource) mountSecondFactor(r chi.Router, authRateLimiter func(http.Handler) http.Handler) {
	if rs.Auth.Routes == nil {
		return
	}
	rs.Auth.Routes.MountSession(r)
	r.Group(func(r chi.Router) {
		if authRateLimiter != nil {
			r.Use(authRateLimiter)
		}
		rs.Auth.Routes.MountCredentialChecks(r)
	})
}
