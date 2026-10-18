package pwa

import (
	"context"
	"net/http"
)

// Usage records a staff session that runs in standalone display mode.
type Usage interface {
	ReportStaff(ctx context.Context, accountID int64) error
}

// UsageRuntime binds the session and the responses the root renders.
type UsageRuntime struct {
	// AccountID is the caller's account, 0 without a session.
	AccountID    func(context.Context) int64
	Unauthorized func(http.ResponseWriter, *http.Request)
	// Failed renders an internal error with the client message.
	Failed    func(w http.ResponseWriter, r *http.Request, message string, err error)
	NoContent func(http.ResponseWriter, *http.Request)
}

// ReportUsage records that the caller's session runs in standalone display
// mode. Repeated reports only advance last_seen_at.
func ReportUsage(usage Usage, runtime UsageRuntime) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accountID := runtime.AccountID(r.Context())
		if accountID == 0 {
			runtime.Unauthorized(w, r)
			return
		}
		if err := usage.ReportStaff(r.Context(), accountID); err != nil {
			runtime.Failed(w, r, "App-Nutzung konnte nicht gespeichert werden.", err)
			return
		}
		runtime.NoContent(w, r)
	})
}
