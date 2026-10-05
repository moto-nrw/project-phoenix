package timetablehttp

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// operationErrorRules classify the refusals of the operational day and the
// planner's lifecycle actions with their own code (#2516). The statuses stay
// as they were; the planner's start-window sentinels carry the same codes on
// both paths.
var operationErrorRules = []common.ErrorRule{
	{Match: isCodedNotFound, Render: codedStatus(common.ErrorNotFoundWithCode)},
	{Target: timetable.ErrTimetableOperationNotFound, Render: notFoundWithCode(common.CodeTimetableInstanceNotFound)},
	{Target: timetable.ErrInstanceNotFound, Render: notFoundWithCode(common.CodeTimetableInstanceNotFound)},
	{Target: timetable.ErrNoStaffProfile, Render: forbiddenWithCode(common.CodeTimetableNoStaffProfile)},
	{Target: timetable.ErrTimetableOperationForbidden, Render: common.ErrorForbidden},
	{Target: timetable.ErrInvalidInstanceTransition, Render: conflictWithCode(common.CodeTimetableInvalidTransition)},
	{Target: timetable.ErrInstanceStartTooEarly, Render: conflictWithCode(common.CodeTimetableStartTooEarly)},
	{Target: timetable.ErrInstanceStartExpired, Render: conflictWithCode(common.CodeTimetableStartWindowExpired)},
	{Target: timetable.ErrInstanceCompleteEarly, Render: conflictWithCode(common.CodeTimetableCompleteTooEarly)},
	{Target: timetable.ErrCompletionConfirmationStale, Render: conflictWithCode(common.CodeTimetableCompletionConfirmationStale)},
	{Target: timetable.ErrTimetableOperationConflict, Render: codedStatus(common.ErrorConflictWithCode, common.CodeTimetableOperationStale)},
	{Target: timetable.ErrInstanceWeekend, Render: invalidWithCode(common.CodeTimetableInstanceWeekend)},
	{Target: timetable.ErrInstanceOutsideActiveCalendarPeriod, Render: invalidWithCode(common.CodeTimetableInstanceOutsidePeriod)},
	// A full room or activity names itself with code and numbers (#3633).
	{Target: studentpresence.ErrRoomCapacityExceeded, Render: common.ErrorBusinessRejectionOr(studentpresence.RoomCapacityCode)},
	{Target: studentpresence.ErrActivityParticipantLimitExceeded, Render: common.ErrorBusinessRejection},
	{Target: studentpresence.ErrStudentAlreadyActive, Render: conflictWithCode(common.CodeTimetableStudentAlreadyActive)},
	{Target: studentpresence.ErrRoomConflict, Render: conflictWithCode(common.CodeTimetableRoomOccupied)},
	{Target: studentpresence.ErrStudentsNotPresent, Render: conflictWithCode(common.CodeTimetableStudentsNotPresent)},
	{Target: studentpresence.ErrGroupAlreadyEnded, Render: conflictWithCode(common.CodeTimetableGroupAlreadyEnded)},
	// A graduated student left on a roster is treated like an unknown one
	// (404), matching the IoT check-in mapper (#405).
	{Target: studentpresence.ErrStudentGraduated, Render: notFoundWithCode(common.CodeTimetableStudentGraduated)},
	{Target: studentpresence.ErrStudentCareEnded, Render: notFoundWithCode(common.CodeTimetableStudentCareEnded)},
	{Target: studentpresence.ErrStudentNotFound, Render: notFoundWithCode(common.CodeTimetableStudentNotFound)},
	{Target: studentpresence.ErrVisitNotFound, Render: notFoundWithCode(common.CodeTimetableStudentNotCheckedIn)},
	{Target: studentpresence.ErrInvalidData, Render: common.ErrorInvalidRequest},
}

func isCodedNotFound(err error) bool {
	_, coded := timetable.AsCoded(err)
	return coded && errors.Is(err, timetable.ErrTimetableOperationNotFound)
}

// codedStatus renders with the code of the CodedError in err's chain, or
// with fallback when err carries none.
func codedStatus(build func(error, string) render.Renderer, fallback ...string) func(error) render.Renderer {
	return func(err error) render.Renderer {
		if coded, ok := timetable.AsCoded(err); ok {
			return build(err, coded.Code)
		}
		if len(fallback) > 0 {
			return build(err, fallback[0])
		}
		return build(err, "")
	}
}

// operationErrorRenderer answers an operational refusal; anything unknown
// is a server error.
func operationErrorRenderer(err error) render.Renderer {
	return common.RenderWithRules(err, operationErrorRules, common.ErrorInternalServer)
}
