package httpadapter

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
)

// scheduleLookupError keeps missing-resource wire messages stable without treating a
// failed read as a missing row.
func scheduleLookupError(err error, notFoundMessage string) render.Renderer {
	if errors.Is(err, timetableModule.ErrDateframeNotFound) ||
		errors.Is(err, timetableModule.ErrTimeframeNotFound) ||
		errors.Is(err, timetableModule.ErrRecurrenceRuleNotFound) {
		return common.ErrorNotFound(errors.New(notFoundMessage))
	}
	return common.ErrorInternalServer(err)
}

// scheduleErrorRules map schedule-service sentinels to HTTP responses. Matched via
// errors.Is against the full error, and rendered WITH the wrapper text —
// this package historically surfaces the "schedule error during {Op}: …"
// prefix (unlike rooms/groups, which strip it), and keeping the bytes
// identical is part of the issue #575 B2 consolidation contract.
var scheduleErrorRules = []common.ErrorRule{
	{Target: timetableModule.ErrDateframeNotFound, Render: common.ErrorNotFound},
	{Target: timetableModule.ErrTimeframeNotFound, Render: common.ErrorNotFound},
	{Target: timetableModule.ErrTimeframeRequiredByCareOffering, Render: common.ErrorConflict},
	{Target: timetableModule.ErrRecurrenceRuleNotFound, Render: common.ErrorNotFound},
	{Target: timetableModule.ErrInvalidRecurrenceRange, Render: common.ErrorInvalidRequest},
	{Target: timetableModule.ErrInvalidTimeRange, Render: common.ErrorInvalidRequest},
	{Target: timetableModule.ErrInvalidDuration, Render: common.ErrorInvalidRequest},
	{Target: studentpresence.ErrRoomCapacityExceeded, Render: common.ErrorConflict},
}

// SchedulesErrorRenderer renders an error to an HTTP response based on the schedule
// service error type.
var SchedulesErrorRenderer = common.RulesRenderer(scheduleErrorRules, common.ErrorInternalServer)
