package active

import (
	"context"

	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
)

// WorkSessionService is the presence-owned port to the Workforce time clock.
// Kiosk-driven attendance toggles and supervision takeovers best-effort-open
// the acting staff member's work session so they show as "Anwesend"
// (#1439). The composition root binds the retained Workforce time-tracking
// service; the presence services never import it.
type WorkSessionService interface {
	// EnsureCheckedIn opens today's session if the staff member has no active
	// one and returns nil when they already checked out today.
	EnsureCheckedIn(ctx context.Context, staffID int64, source string) (*activeModels.WorkSession, error)
}

// PlannedStartNotReachedError classifies the time clock's refusal to open a
// session before the planned start. The Workforce time clock returns its own
// error value; presence matches it by this shape so the auto-check-in can
// skip quietly instead of logging a failure.
type PlannedStartNotReachedError interface {
	error
	// PlannedStartNotReached returns the planned start and the current
	// wall-clock time the time clock compared, both formatted as HH:MM.
	PlannedStartNotReached() (plannedStartTime, currentTime string)
}
