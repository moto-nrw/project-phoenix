package timetracking

import (
	"errors"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// Error rules classify owner identities; the first matching rule wins.

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
	{Target: workforce.ErrAlreadyCheckedIn, Render: conflictWithCode(common.CodeWorkforceAlreadyCheckedIn)},
	{Target: workforce.ErrBreakAlreadyActive, Render: conflictWithCode(common.CodeWorkforceBreakAlreadyActive)},
	{Target: workforce.ErrAlreadyCheckedOut, Render: conflictWithCode(common.CodeWorkforceAlreadyCheckedOut)},
	// The overlap message carries the conflicting interval, so only the code
	// identifies it.
	{Target: workforce.ErrWorkSessionOverlap, Render: conflictWithCode(common.CodeWorkforceWorkSessionOverlap)},
	{Target: workforce.ErrNoActiveSession, Render: notFoundWithCode(common.CodeWorkforceNoActiveSession)},
	{Target: workforce.ErrNoActiveBreak, Render: notFoundWithCode(common.CodeWorkforceNoActiveBreak)},
	{Target: workforce.ErrNoSessionForToday, Render: notFoundWithCode(common.CodeWorkforceNoSessionForToday)},
	{Target: workforce.ErrSessionNotFound, Render: notFoundWithCode(common.CodeWorkforceSessionNotFound)},
	{Match: isAny(workforce.ErrSessionUpdateNotOwned, workforce.ErrSessionNotOwned), Render: forbiddenWithCode(common.CodeWorkforceSessionNotOwned)},
	// Only these Validate() failures describe the recorded times; a missing
	// staff ID or creator is a server fault and stays a 500.
	{Match: isAny(workforce.ErrSessionTimeOrder, workforce.ErrBreakMinutesNegative, workforce.ErrSessionCheckoutOrder, workforce.ErrBreakDurationNegative), Render: invalidWithCode(common.CodeWorkforceSessionTimesInvalid)},
	{Match: isAny(workforce.ErrSessionStatusNoteRequired, workforce.ErrSessionTimesNoteRequired), Render: invalidWithCode(common.CodeWorkforceSessionNoteRequired)},
	{Target: workforce.ErrSessionStatusInvalid, Render: common.ErrorInvalidRequest},
	{Target: workforce.ErrSessionSourceInvalid, Render: common.ErrorInvalidRequest},
	{Target: workforce.ErrPlannedBreakDurationInvalid, Render: common.ErrorInvalidRequest},
	{Target: workforce.ErrBreakNotOwned, Render: common.ErrorInvalidRequest},
	{Target: workforce.ErrActiveBreakDurationEdit, Render: common.ErrorInvalidRequest},
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
	{Target: workforce.ErrAbsenceNotFound, Render: notFoundWithCode(common.CodeWorkforceAbsenceNotFound)},
	{Match: isAny(workforce.ErrAbsenceUpdateNotOwned, workforce.ErrAbsenceDeleteNotOwned, workforce.ErrAbsenceCancelNotOwned, workforce.ErrAbsenceResubmitNotOwned), Render: forbiddenWithCode(common.CodeWorkforceAbsenceNotOwned)},
	{Match: isAny(workforce.ErrAbsenceNotCancelable, workforce.ErrPastAbsenceCancel), Render: conflictWithCode(common.CodeWorkforceAbsenceNotCancelable)},
	{Target: workforce.ErrAbsenceNoWorkingDays, Render: invalidWithCode(common.CodeWorkforceAbsenceNoWorkingDays)},
	{Target: workforce.ErrVacationRequestInPast, Render: invalidWithCode(common.CodeWorkforceVacationRequestInPast)},
	{Target: workforce.ErrAbsenceResubmitNoteRequired, Render: common.ErrorInvalidRequest},
	// The Leitung decided while the person was still answering the question.
	{Target: workforce.ErrAbsenceNotQuestioned, Render: invalidWithCode(common.CodeWorkforceAbsenceAlreadyDecided)},
	{Target: workforce.ErrAbsenceOverlap, Render: conflictWithCode(common.CodeWorkforceAbsenceOverlap)},
	{Target: workforce.ErrAbsenceDatesOverlap, Render: conflictWithCode(common.CodeWorkforceAbsenceOverlap)},
	{Target: workforce.ErrAbsenceUpdatedDatesOverlap, Render: conflictWithCode(common.CodeWorkforceAbsenceOverlap)},
	{Target: workforce.ErrAbsenceInvalid, Render: common.ErrorInvalidRequest},
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

func isAny(targets ...error) func(error) bool {
	return func(err error) bool {
		for _, target := range targets {
			if errors.Is(err, target) {
				return true
			}
		}
		return false
	}
}

func notFoundWithCode(code string) func(error) render.Renderer {
	return func(err error) render.Renderer { return common.ErrorNotFoundWithCode(err, code) }
}

func forbiddenWithCode(code string) func(error) render.Renderer {
	return func(err error) render.Renderer { return common.ErrorForbiddenWithCode(err, code) }
}
