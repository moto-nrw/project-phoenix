package timetable

import (
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ErrLifecycleSettings reports that the tenant's lifecycle settings (start
// lead, planned-end policy) could not be resolved.
var ErrLifecycleSettings = errors.New("activity lifecycle settings unavailable")

// LifecycleWindow is the planned window of one block: its date, the
// wall-clock start and end, and whether it was started ad hoc.
type LifecycleWindow struct {
	Date          calendar.Date
	StartTime     time.Time
	EndTime       time.Time
	IsSpontaneous bool
}

// LifecycleAvailability says when a block may start and complete.
type LifecycleAvailability struct {
	CanStart            bool
	StartAvailableAt    time.Time
	CanComplete         bool
	CompleteAvailableAt time.Time
}

// LifecycleBoundary is the Berlin instant of a wall-clock time on the block's
// date.
func LifecycleBoundary(day calendar.Date, wallClock time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), wallClock.Hour(), wallClock.Minute(), wallClock.Second(), wallClock.Nanosecond(), calendar.Berlin)
}

// EvaluateLifecycleAvailability is the shared clock policy of the lifecycle
// writes and the payloads that announce them. A planned block starts from
// the configured lead before its start until (but not including) its planned
// end, and completes from its planned end when the tenant enforces it.
// Spontaneous blocks are not bound to plan times.
func EvaluateLifecycleAvailability(window LifecycleWindow, now time.Time, startLeadMinutes int, enforcePlannedEnd bool) LifecycleAvailability {
	if window.IsSpontaneous {
		return LifecycleAvailability{CanStart: true, StartAvailableAt: now, CanComplete: true, CompleteAvailableAt: now}
	}
	start := LifecycleBoundary(window.Date, window.StartTime)
	end := LifecycleBoundary(window.Date, window.EndTime)
	availableAt := start.Add(-time.Duration(startLeadMinutes) * time.Minute)
	return LifecycleAvailability{
		CanStart:            !now.Before(availableAt) && now.Before(end),
		StartAvailableAt:    availableAt,
		CanComplete:         !enforcePlannedEnd || !now.Before(end),
		CompleteAvailableAt: end,
	}
}

// SubstituteDayLockKey is the one advisory-lock key of the day-wide staffing
// mutations of a tenant (#1840): substitutions, deviations, the sick cascade
// and the re-plan of a week all lock the day with it. A substitution or
// absence is day-wide, so locking one block would let two admins editing
// different blocks of the same absent person both place a substitute. Every
// caller must hash the same string, or they would not contend.
func SubstituteDayLockKey(tenantID int64, date calendar.Date) string {
	return fmt.Sprintf("timetable:substitute-day:%d:%s", tenantID, date.String())
}

// CanReopenAsActor is the actor/admin half of the reopen gate: a completed
// block reopens for an admin or for the account that completed it.
// Completing a live group ends its supervisor row, so operational access is
// not a substitute for this check.
func CanReopenAsActor(completed bool, completedBy *int64, accountID int64, isAdmin bool) bool {
	if !completed {
		return false
	}
	if isAdmin {
		return true
	}
	return completedBy != nil && *completedBy == accountID
}
