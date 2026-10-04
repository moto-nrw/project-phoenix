package timetracking

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// The web time-tracking routes classify through the rule tables below; the
// first matching rule wins. The legacy service names many failures only by
// their wording, and that wording also feeds the IoT stamp mapping
// (services/workforce_time_clock.go), so it stays as it is. The web answer
// carries a stable code, so the frontend never reads the sentence (#2514).

// classifyServiceError maps known business errors to appropriate HTTP status codes
func classifyServiceError(err error) render.Renderer {
	return common.RenderWithRules(err, serviceErrorRules, common.ErrorInternalServer)
}

// classifyAbsenceError maps known absence business errors to HTTP status codes
func classifyAbsenceError(err error) render.Renderer {
	return common.RenderWithRules(err, absenceErrorRules, common.ErrorInternalServer)
}

var serviceErrorRules = []common.ErrorRule{
	{Match: isPlannedStartNotReached, Render: renderPlannedStartNotReached},
	// Typed F9 deviation gate: 409 with a stable code plus the deviation
	// facts, so the frontend can ask for a reason and retry the same stamp.
	{Match: isDeviationReasonRequired, Render: renderDeviationReasonRequired},
	// A concurrent stamp of the same person lost the race: to the person it
	// reads exactly like stamping twice.
	{Target: workforce.ErrCheckInRaced, Render: conflictWithCode(common.CodeWorkforceAlreadyCheckedIn)},
	{Match: msgIs("already checked in"), Render: conflictWithCode(common.CodeWorkforceAlreadyCheckedIn)},
	{Match: msgIs("break already active"), Render: conflictWithCode(common.CodeWorkforceBreakAlreadyActive)},
	{Match: msgIs("already checked out today"), Render: common.ErrorConflict},
	// The overlap message carries the conflicting interval, so only the code
	// identifies it.
	{Match: msgPrefix("work session overlaps an existing block"), Render: conflictWithCode(common.CodeWorkforceWorkSessionOverlap)},
	{Match: msgIs("no active session found"), Render: notFoundWithCode(common.CodeWorkforceNoActiveSession)},
	{Match: msgIs("no active break found"), Render: notFoundWithCode(common.CodeWorkforceNoActiveBreak)},
	{Match: msgIn("no session found for today", "session not found"), Render: common.ErrorNotFound},
	{Match: msgIn("can only update own sessions", "session does not belong to requesting staff"), Render: common.ErrorForbidden},
	// Only these Validate() failures describe the recorded times; a missing
	// staff ID or creator is a server fault and stays a 500.
	{Match: msgIn(
		"invalid session data: check-in time must be before check-out time",
		"invalid session data: break minutes cannot be negative",
		"check_out_time must be after check_in_time",
		"break minutes cannot be negative",
		"break duration cannot be negative",
	), Render: invalidWithCode(common.CodeWorkforceSessionTimesInvalid)},
	{Match: msgIn("notes required when changing status", "notes required when changing recorded times"), Render: invalidWithCode(common.CodeWorkforceSessionNoteRequired)},
	{Match: msgPrefix("status must be"), Render: common.ErrorInvalidRequest},
	{Match: msgPrefix("source must be"), Render: common.ErrorInvalidRequest},
	{Match: msgPrefix("planned_duration_minutes must be"), Render: common.ErrorInvalidRequest},
	{Match: isForeignBreak, Render: common.ErrorInvalidRequest},
	{Match: msgIs("cannot edit duration of an active break"), Render: common.ErrorInvalidRequest},
}

var absenceErrorRules = []common.ErrorRule{
	{Target: workforce.ErrManagerControlledAbsence, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, common.CodeWorkforceManagerControlledAbsence)
	}},
	// School-defined Abwesenheitsarten (#2403). A retired or unknown art is a
	// bad selection, not a server fault — the client has to pick another one.
	{Target: workforce.ErrAbsenceTypeInactive, Render: conflictWithCode(common.CodeWorkforceAbsenceTypeInactive)},
	{Target: workforce.ErrAbsenceTypeNotFound, Render: common.ErrorInvalidRequest},
	// Kontingente may not go negative (#3256).
	{Target: workforce.ErrVacationQuotaExceeded, Render: conflictWithCode(common.CodeWorkforceVacationQuotaExceeded)},
	{Target: workforce.ErrAllowanceBookingOverlap, Render: conflictWithCode(common.CodeWorkforceAbsenceOverlap)},
	{Match: msgIs("absence not found"), Render: common.ErrorNotFound},
	{Match: msgIn(
		"can only update own absences",
		"can only delete own absences",
		"can only cancel own absences",
		"can only resubmit own absences",
	), Render: common.ErrorForbidden},
	{Match: msgIn("only pending or approved absences can be canceled", "past absences cannot be canceled"), Render: conflictWithCode(common.CodeWorkforceAbsenceNotCancelable)},
	{Match: msgIs("vacation range contains no working days"), Render: invalidWithCode(common.CodeWorkforceAbsenceNoWorkingDays)},
	{Match: msgIs("vacation request must start today or in the future"), Render: invalidWithCode(common.CodeWorkforceVacationRequestInPast)},
	{Match: msgIs("resubmit note is required"), Render: common.ErrorInvalidRequest},
	// The Leitung decided while the person was still answering the question.
	{Match: msgIs("only absences with a question can be resubmitted"), Render: invalidWithCode(common.CodeWorkforceAbsenceAlreadyDecided)},
	{Match: msgPrefix("absence overlaps"), Render: conflictWithCode(common.CodeWorkforceAbsenceOverlap)},
	{Match: msgPrefix("dates overlap"), Render: conflictWithCode(common.CodeWorkforceAbsenceOverlap)},
	{Match: msgPrefix("updated dates overlap"), Render: conflictWithCode(common.CodeWorkforceAbsenceOverlap)},
	{Match: msgPrefix("invalid"), Render: common.ErrorInvalidRequest},
	// The #1843 sick cascade wraps schedule-layer errors: reactivating a
	// shift whose freed window was re-planned collides as an overlap.
	{Target: workforce.ErrStaffShiftOverlap, Render: conflictWithCode(common.CodeWorkforceShiftOverlap)},
}

func isPlannedStartNotReached(err error) bool {
	var target *workforce.PlannedStartNotReachedError
	return errors.As(err, &target)
}

func renderPlannedStartNotReached(err error) render.Renderer {
	var plannedStart *workforce.PlannedStartNotReachedError
	errors.As(err, &plannedStart)
	return common.ErrorConflictWithDetails(err, common.CodeIotPlannedStartNotReached, map[string]any{
		"planned_start_time": plannedStart.PlannedStartTime,
		"current_time":       plannedStart.CurrentTime,
	})
}

func isDeviationReasonRequired(err error) bool {
	var target *workforce.DeviationReasonRequiredError
	return errors.As(err, &target)
}

func renderDeviationReasonRequired(err error) render.Renderer {
	var deviation *workforce.DeviationReasonRequiredError
	errors.As(err, &deviation)
	return common.ErrorConflictWithDetails(err, common.CodeIotDeviationReasonRequired, map[string]any{
		"action":            deviation.Action,
		"planned_time":      deviation.PlannedTime,
		"actual_time":       deviation.ActualTime,
		"deviation_minutes": strconv.Itoa(deviation.DeviationMinutes),
	})
}

// isForeignBreak matches "break <id> does not belong to this session".
func isForeignBreak(err error) bool {
	msg := err.Error()
	return strings.HasPrefix(msg, "break ") && strings.Contains(msg, "does not belong to this session")
}

func msgIn(messages ...string) func(error) bool {
	return func(err error) bool { return slices.Contains(messages, err.Error()) }
}

func notFoundWithCode(code string) func(error) render.Renderer {
	return func(err error) render.Renderer { return common.ErrorNotFoundWithCode(err, code) }
}
