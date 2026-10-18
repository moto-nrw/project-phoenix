package common

import (
	"context"
	"errors"
	"net/http"
)

// RequireWebAttendanceEnabled makes attendance.web_enabled authoritative for
// staff-initiated HTTP mutations. It must run after TenantTxMiddleware so the
// setting is resolved in the request tenant. IoT, scheduler, and other system
// paths do not use this middleware and remain unaffected.
func RequireWebAttendanceEnabled(enabledForRequest func(context.Context) (bool, error)) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if enabledForRequest == nil {
				RenderError(w, r, ErrorServiceUnavailable(errors.New("attendance settings service is not configured")))
				return
			}
			enabled, err := enabledForRequest(r.Context())
			if err != nil {
				RenderError(w, r, ErrorServiceUnavailable(err))
				return
			}
			if !enabled {
				RenderError(w, r, ErrorForbiddenWithCode(errors.New("web attendance is disabled"), CodeAttendanceWebDisabled))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
