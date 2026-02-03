package timetracking

import (
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// classifyServiceError maps known business errors to appropriate HTTP status codes
func classifyServiceError(err error) render.Renderer {
	msg := err.Error()

	switch {
	case msg == "already checked in",
		msg == "already checked out today",
		msg == "break already active":
		return common.ErrorConflict(err)

	case msg == "no active session found",
		msg == "no session found for today",
		msg == "session not found",
		msg == "no active break found":
		return common.ErrorNotFound(err)

	case msg == "can only update own sessions",
		msg == "session does not belong to requesting staff":
		return common.ErrorForbidden(err)

	case strings.HasPrefix(msg, "status must be"),
		msg == "break minutes cannot be negative",
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
	case msg == "absence not found":
		return common.ErrorNotFound(err)

	case msg == "can only update own absences",
		msg == "can only delete own absences":
		return common.ErrorForbidden(err)

	case strings.HasPrefix(msg, "absence overlaps"),
		strings.HasPrefix(msg, "updated dates overlap"):
		return common.ErrorConflict(err)

	case strings.HasPrefix(msg, "invalid"),
		msg == "invalid absence type",
		msg == "invalid absence status":
		return common.ErrorInvalidRequest(err)

	default:
		return common.ErrorInternalServer(err)
	}
}
