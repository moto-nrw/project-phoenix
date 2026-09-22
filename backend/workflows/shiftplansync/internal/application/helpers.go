package application

import (
	"context"
	"fmt"
	"log/slog"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The package-private helpers the workflow shares with the retained timetable
// services. modules/timetable/legacy/timetableplanning (#3218) keeps its own
// copies for the instance, deviation and materialization paths; these came
// along with the two writes that use them (#3418).

type legacyListRepository[T any] interface {
	List(context.Context, *modelBase.QueryOptions) ([]T, error)
}

func legacyList[T any](ctx context.Context, repository any, options *modelBase.QueryOptions) ([]T, error) {
	lister, ok := repository.(legacyListRepository[T])
	if !ok {
		return nil, fmt.Errorf("legacy list capability is not configured for %T", repository)
	}
	return lister.List(ctx, options)
}

// broadcastStaffingChanged queues one tenant-wide staffing_deviation_changed
// event after the surrounding tenant transaction commits (outside a tx it
// fires immediately). Every write that changes who is planned where must
// emit it, or the planner and "Heute geplant" card go stale until reload
// (#1844). A nil broadcaster is a no-op (unit tests, CLI wiring).
func broadcastStaffingChanged(ctx context.Context, broadcaster realtime.Broadcaster, logger *slog.Logger, source string) {
	if broadcaster == nil {
		return
	}
	tenantID := tenant.FromContext(ctx)
	event := realtime.NewEvent(realtime.EventStaffingDeviationChanged, "", realtime.EventData{Source: &source})
	tenant.RegisterAfterCommit(ctx, func() {
		if err := broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			logger.Warn("SSE planned instance broadcast failed",
				slog.String("source", source),
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		}
	})
}

// isPlannableInstance reports whether a substitute/absence write may touch this
// instance. Only planned and active blocks are editable; completed and cancelled
// ones are historical record.
func isPlannableInstance(instance *scheduleModel.ActivityInstance) bool {
	return instance.Status == scheduleModel.InstanceStatusPlanned ||
		instance.Status == scheduleModel.InstanceStatusActive
}
