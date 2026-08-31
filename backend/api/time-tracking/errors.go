package timetracking

import (
	"errors"
	"strconv"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// classifyServiceError maps known business errors to appropriate HTTP status codes
func classifyServiceError(err error) render.Renderer {
	var plannedStart *activeSvc.PlannedStartNotReachedError
	if errors.As(err, &plannedStart) {
		return common.ErrorConflictWithDetails(err, "planned_start_not_reached", map[string]any{
			"planned_start_time": plannedStart.PlannedStartTime,
			"current_time":       plannedStart.CurrentTime,
		})
	}

	// Typed F9 deviation gate: 409 with a stable code plus the deviation
	// facts, so the frontend can prompt "Du stempelst N Minuten nach
	// Planende aus" and retry the same stamp with a reason.
	var deviation *activeSvc.DeviationReasonRequiredError
	if errors.As(err, &deviation) {
		return common.ErrorConflictWithDetails(err, "deviation_reason_required", map[string]any{
			"action":            deviation.Action,
			"planned_time":      deviation.PlannedTime,
			"actual_time":       deviation.ActualTime,
			"deviation_minutes": strconv.Itoa(deviation.DeviationMinutes),
		})
	}

	msg := err.Error()

	switch {
	case msg == "already checked in",
		msg == "already checked out today",
		msg == "break already active":
		return common.ErrorConflict(err)

	// The overlap message carries the dynamic conflicting interval, so the
	// frontend cannot use it as a mapping key — the stable code drives the
	// specific German toast instead of the generic stamp error.
	case strings.HasPrefix(msg, "work session overlaps an existing block"):
		return common.ErrorConflictWithCode(err, "work_session_overlap")

	case msg == "no active session found",
		msg == "no session found for today",
		msg == "session not found",
		msg == "no active break found":
		return common.ErrorNotFound(err)

	case msg == "can only update own sessions",
		msg == "session does not belong to requesting staff":
		return common.ErrorForbidden(err)

	case strings.HasPrefix(msg, "status must be"),
		strings.HasPrefix(msg, "source must be"),
		msg == "notes required when changing status",
		msg == "notes required when changing recorded times",
		msg == "break minutes cannot be negative",
		msg == "break duration cannot be negative",
		msg == "invalid session data: check-in time must be before check-out time",
		msg == "check_out_time must be after check_in_time",
		strings.HasPrefix(msg, "planned_duration_minutes must be"),
		strings.HasPrefix(msg, "break ") && strings.Contains(msg, "does not belong to this session"),
		msg == "cannot edit duration of an active break":
		return common.ErrorInvalidRequest(err)

	default:
		return common.ErrorInternalServer(err)
	}
}

// classifyAbsenceError maps known absence business errors to HTTP status codes
func classifyAbsenceError(err error) render.Renderer {
	msg := err.Error()

	switch {
	case errors.Is(err, activeSvc.ErrManagerControlledAbsence):
		return common.ErrorForbiddenWithCode(err, "manager_controlled_absence")

	// School-defined Abwesenheitsarten (#2403). A retired or unknown art is a
	// bad selection, not a server fault — the client has to pick another one.
	case errors.Is(err, activeSvc.ErrAbsenceTypeInactive):
		return common.ErrorConflictWithCode(err, "absence_type_inactive")

	case errors.Is(err, activeSvc.ErrAbsenceTypeNotFound):
		return common.ErrorInvalidRequest(err)

	case msg == "absence not found":
		return common.ErrorNotFound(err)

	case msg == "can only update own absences",
		msg == "can only delete own absences",
		msg == "can only resubmit own absences":
		return common.ErrorForbidden(err)

	case msg == "resubmit note is required",
		msg == "only absences with a question can be resubmitted":
		return common.ErrorInvalidRequest(err)

	case strings.HasPrefix(msg, "absence overlaps"),
		strings.HasPrefix(msg, "dates overlap"),
		strings.HasPrefix(msg, "updated dates overlap"):
		return common.ErrorConflict(err)

	case strings.HasPrefix(msg, "invalid"),
		msg == "invalid absence type",
		msg == "invalid absence status":
		return common.ErrorInvalidRequest(err)

	// The #1843 sick cascade wraps schedule-layer errors: reactivating a
	// shift whose freed window was re-planned collides as an overlap.
	case errors.Is(err, scheduleSvc.ErrShiftOverlap):
		return common.ErrorConflict(err)

	default:
		return common.ErrorInternalServer(err)
	}
}
