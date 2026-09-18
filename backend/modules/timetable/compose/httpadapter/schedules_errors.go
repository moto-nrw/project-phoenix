package httpadapter

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan/legacy/careschedule"
)

// scheduleLookupError keeps missing-resource wire messages stable without treating a
// failed read as a missing row.
func scheduleLookupError(err error, notFoundMessage string) render.Renderer {
	if errors.Is(err, careschedule.ErrDateframeNotFound) ||
		errors.Is(err, careschedule.ErrTimeframeNotFound) ||
		errors.Is(err, careschedule.ErrRecurrenceRuleNotFound) {
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
	{Target: careschedule.ErrDateframeNotFound, Render: common.ErrorNotFound},
	{Target: careschedule.ErrTimeframeNotFound, Render: common.ErrorNotFound},
	{Target: careschedule.ErrTimeframeRequiredByCareOffering, Render: common.ErrorConflict},
	{Target: careschedule.ErrRecurrenceRuleNotFound, Render: common.ErrorNotFound},
	{Target: careschedule.ErrInvalidDateRange, Render: common.ErrorInvalidRequest},
	{Target: careschedule.ErrInvalidTimeRange, Render: common.ErrorInvalidRequest},
	{Target: careschedule.ErrInvalidDuration, Render: common.ErrorInvalidRequest},
	{Target: careschedule.ErrRoomCapacityExceeded, Render: common.ErrorConflict},
}

// SchedulesErrorRenderer renders an error to an HTTP response based on the schedule
// service error type.
var SchedulesErrorRenderer = common.RulesRenderer(scheduleErrorRules, common.ErrorInternalServer)
