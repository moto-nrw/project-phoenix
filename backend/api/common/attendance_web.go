package common

import (
	"context"
	"errors"
	"net/http"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

const ErrCodeAttendanceWebDisabled = "attendance.web_disabled"

// RequireWebAttendanceEnabled makes attendance.web_enabled authoritative for
// staff-initiated HTTP mutations. It must run after TenantTxMiddleware so the
// setting is resolved in the request tenant. IoT, scheduler, and other system
// paths do not use this middleware and remain unaffected.
func RequireWebAttendanceEnabled(settings interface {
	ResolveBool(context.Context, string) (bool, error)
}) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if settings == nil {
				RenderError(w, r, ErrorServiceUnavailable(errors.New("attendance settings service is not configured")))
				return
			}
			enabled, err := settings.ResolveBool(r.Context(), configModel.KeyAttendanceWebEnabled)
			if err != nil {
				RenderError(w, r, ErrorServiceUnavailable(err))
				return
			}
			if !enabled {
				RenderError(w, r, ErrorForbiddenWithCode(errors.New("web attendance is disabled"), ErrCodeAttendanceWebDisabled))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
