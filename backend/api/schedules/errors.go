package schedules

import (
	"github.com/moto-nrw/project-phoenix/api/common"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// errorRules map schedule-service sentinels to HTTP responses. Matched via
// errors.Is against the full error, and rendered WITH the wrapper text —
// this package historically surfaces the "schedule error during {Op}: …"
// prefix (unlike rooms/groups, which strip it), and keeping the bytes
// identical is part of the issue #575 B2 consolidation contract.
var errorRules = []common.ErrorRule{
	{Target: scheduleSvc.ErrDateframeNotFound, Render: common.ErrorNotFound},
	{Target: scheduleSvc.ErrTimeframeNotFound, Render: common.ErrorNotFound},
	{Target: scheduleSvc.ErrTimeframeRequiredByCareOffering, Render: common.ErrorConflict},
	{Target: scheduleSvc.ErrRecurrenceRuleNotFound, Render: common.ErrorNotFound},
	{Target: scheduleSvc.ErrInvalidDateRange, Render: common.ErrorInvalidRequest},
	{Target: scheduleSvc.ErrInvalidTimeRange, Render: common.ErrorInvalidRequest},
	{Target: scheduleSvc.ErrInvalidDuration, Render: common.ErrorInvalidRequest},
	{Target: activeSvc.ErrRoomCapacityExceeded, Render: common.ErrorConflict},
}

// ErrorRenderer renders an error to an HTTP response based on the schedule
// service error type.
var ErrorRenderer = common.RulesRenderer(errorRules, common.ErrorInternalServer)
