package shiftplanning

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The package-private helpers the moved services shared with the retained
// timetable services. modules/timetable/legacy/timetableplanning (#3218) keeps
// its own copies for the instance, deviation and materialization paths; these
// go with the services that use them (#3219).

// isoWeekday returns the ISO 8601 weekday number for d (1=Mon … 7=Sun),
// matching the storage convention of activities.schedules.weekday.
func isoWeekday(d timezone.Date) int {
	wd := d.Weekday()
	if wd == time.Sunday {
		return 7
	}
	return int(wd)
}

func marshalDeviationValue(v any) (json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

// normalizeActor maps a missing/zero actor to nil so the row stores NULL
// instead of a fabricated id.
func normalizeActor(actorAccountID *int64) *int64 {
	if actorAccountID == nil || *actorAccountID <= 0 {
		return nil
	}
	return actorAccountID
}

// int64FilterArgs widens IDs for the persistence-neutral Filter.In API.
func int64FilterArgs(ids []int64) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}

type legacyListRepository[T any] interface {
	List(context.Context, *modelBase.QueryOptions) ([]T, error)
}

type legacyListWithOptionsRepository[T any] interface {
	ListWithOptions(context.Context, *modelBase.QueryOptions) ([]T, error)
}

func legacyList[T any](ctx context.Context, repository any, options *modelBase.QueryOptions) ([]T, error) {
	lister, ok := repository.(legacyListRepository[T])
	if !ok {
		return nil, fmt.Errorf("legacy list capability is not configured for %T", repository)
	}
	return lister.List(ctx, options)
}

func legacyListWithOptions[T any](ctx context.Context, repository any, options *modelBase.QueryOptions) ([]T, error) {
	lister, ok := repository.(legacyListWithOptionsRepository[T])
	if !ok {
		return nil, fmt.Errorf("legacy list-with-options capability is not configured for %T", repository)
	}
	return lister.ListWithOptions(ctx, options)
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
