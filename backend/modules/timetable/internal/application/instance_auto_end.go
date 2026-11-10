package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	scheduleModel "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type instanceAutoEnd struct {
	instances  ports.LegacyInstanceReads
	completion timetable.InstanceLifecycle
	logger     *slog.Logger
}

// NewInstanceAutoEnd composes the tenant-scoped automatic completion, which
// completes through the lifecycle the manual completion uses. A nil logger
// falls back to slog.Default().
func NewInstanceAutoEnd(instances ports.LegacyInstanceReads, completion timetable.InstanceLifecycle, logger *slog.Logger) (timetable.InstanceAutoEnd, error) {
	if instances == nil || completion == nil {
		return nil, errors.New("timetable auto-end: instances and lifecycle are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &instanceAutoEnd{instances: instances, completion: completion, logger: logger}, nil
}

// RunForTenant completes every running block of the tenant whose planned
// end plus grace has passed. A failed completion does not stop the others;
// it stays in the result, is logged with its cause and is retried on the next
// tick. A savepoint-control failure aborts the run: the surrounding
// transaction is unusable, so every later block would fail as well.
func (s *instanceAutoEnd) RunForTenant(ctx context.Context, now time.Time, grace time.Duration) (*timetable.AutoEndResult, error) {
	startedAt := time.Now()
	result := &timetable.AutoEndResult{}
	defer func() {
		result.DurationMS = time.Since(startedAt).Milliseconds()
	}()
	instances, err := listLegacyInstances(ctx, s.instances, ports.LegacyInstanceFilter{Status: scheduleModel.InstanceStatusActive}, "list with options")
	if err != nil {
		return result, fmt.Errorf("load active activity instances: %w", err)
	}
	// Successful completions keep their day locks until the batch commits.
	slices.SortStableFunc(instances, func(a, b *scheduleModel.ActivityInstance) int {
		return a.Date.Compare(b.Date)
	})
	for _, instance := range instances {
		result.Checked++
		err := s.completeIfDueIsolated(ctx, instance, now, grace, result)
		if err == nil {
			continue
		}
		if errors.Is(err, tenant.ErrSavepointControl) {
			return result, fmt.Errorf("auto-end activity instance %d: %w", instance.ID, err)
		}
		s.logger.Warn("timetable auto-end could not complete activity instance",
			slog.Int64("instance_id", instance.ID),
			slog.String("error", err.Error()),
		)
	}
	return result, nil
}

// completeIfDueIsolated rolls back only this completion when the scheduler
// already runs in a tenant transaction: nested transactions map to
// savepoints, so a failed block does not abort the batch. Blocks that are not
// due are counted from the listed row before any savepoint: they issue no
// statement, so there is nothing to isolate (#5809).
func (s *instanceAutoEnd) completeIfDueIsolated(ctx context.Context, instance *scheduleModel.ActivityInstance, now time.Time, grace time.Duration, result *timetable.AutoEndResult) error {
	if skipNotDue(instance, now, grace, result) {
		return nil
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return s.complete(ctx, instance.ID, result)
	}
	return tenant.WithSavepoint(ctx, func(savepointCtx context.Context) error {
		return s.complete(savepointCtx, instance.ID, result)
	})
}

// skipNotDue counts a listed block that auto-end leaves running: not active,
// spontaneous, or before its planned end plus grace. It reads only the row.
func skipNotDue(instance *scheduleModel.ActivityInstance, now time.Time, grace time.Duration, result *timetable.AutoEndResult) bool {
	if instance.Status != scheduleModel.InstanceStatusActive {
		result.SkippedNonActive++
		return true
	}
	if instance.IsSpontaneous {
		result.SkippedSpontaneous++
		return true
	}
	deadline := timetable.LifecycleBoundary(calendar.Date(instance.Date), instance.EndTime).Add(grace)
	if now.Before(deadline) {
		result.SkippedBeforeDeadline++
		return true
	}
	return false
}

func (s *instanceAutoEnd) complete(ctx context.Context, instanceID int64, result *timetable.AutoEndResult) error {
	if _, err := s.completion.Complete(ctx, instanceID); err != nil {
		return s.classifyCompletionFailure(ctx, instanceID, err, result)
	}
	result.Completed++
	return nil
}

// classifyCompletionFailure counts a completion another writer got to first
// as concurrent and everything else as failed.
func (s *instanceAutoEnd) classifyCompletionFailure(ctx context.Context, instanceID int64, err error, result *timetable.AutoEndResult) error {
	if errors.Is(err, timetable.ErrInstanceMoved) || errors.Is(err, timetable.ErrInstanceNotFound) {
		result.SkippedConcurrent++
		return nil
	}
	if errors.Is(err, timetable.ErrInvalidInstanceTransition) {
		current, findErr := findLegacyInstance(ctx, s.instances, instanceID)
		if findErr != nil {
			if domain.IsRecordNotFound(findErr) {
				result.SkippedConcurrent++
				return nil
			}
			result.Failed++
			return fmt.Errorf("verify concurrent completion: %w", findErr)
		}
		if current == nil || current.Status != scheduleModel.InstanceStatusActive {
			result.SkippedConcurrent++
			return nil
		}
	}
	result.Failed++
	return err
}
