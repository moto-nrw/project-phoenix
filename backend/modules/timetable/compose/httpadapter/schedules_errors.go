package httpadapter

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// scheduleLookupError keeps missing-resource wire messages stable without treating a
// failed read as a missing row.
func scheduleLookupError(err error, notFoundMessage string) render.Renderer {
	if errors.Is(err, scheduleSvc.ErrDateframeNotFound) ||
		errors.Is(err, scheduleSvc.ErrTimeframeNotFound) ||
		errors.Is(err, scheduleSvc.ErrRecurrenceRuleNotFound) {
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
	{Target: scheduleSvc.ErrDateframeNotFound, Render: common.ErrorNotFound},
	{Target: scheduleSvc.ErrTimeframeNotFound, Render: common.ErrorNotFound},
	{Target: scheduleSvc.ErrTimeframeRequiredByCareOffering, Render: common.ErrorConflict},
	{Target: scheduleSvc.ErrRecurrenceRuleNotFound, Render: common.ErrorNotFound},
	{Target: scheduleSvc.ErrInvalidDateRange, Render: common.ErrorInvalidRequest},
	{Target: scheduleSvc.ErrInvalidTimeRange, Render: common.ErrorInvalidRequest},
	{Target: scheduleSvc.ErrInvalidDuration, Render: common.ErrorInvalidRequest},
	{Target: activeModels.ErrRoomCapacityExceeded, Render: common.ErrorConflict},
}

// SchedulesErrorRenderer renders an error to an HTTP response based on the schedule
// service error type.
var SchedulesErrorRenderer = common.RulesRenderer(scheduleErrorRules, common.ErrorInternalServer)
