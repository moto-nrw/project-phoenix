package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type instanceAutoEnd struct {
	instances  scheduleModel.ActivityInstanceRepository
	completion timetable.InstanceLifecycle
}

// NewInstanceAutoEnd composes the tenant-scoped automatic completion, which
// completes through the lifecycle the manual completion uses.
func NewInstanceAutoEnd(instances scheduleModel.ActivityInstanceRepository, completion timetable.InstanceLifecycle) (timetable.InstanceAutoEnd, error) {
	if instances == nil || completion == nil {
		return nil, errors.New("timetable auto-end: instances and lifecycle are required")
	}
	return &instanceAutoEnd{instances: instances, completion: completion}, nil
}

// RunForTenant completes every running block of the tenant whose planned
// end plus grace has passed. A failed completion does not stop the others;
// it stays in the result and is retried on the next tick.
func (s *instanceAutoEnd) RunForTenant(ctx context.Context, now time.Time, grace time.Duration) (*timetable.AutoEndResult, error) {
	startedAt := time.Now()
	result := &timetable.AutoEndResult{}
	defer func() {
		result.DurationMS = time.Since(startedAt).Milliseconds()
	}()
	options := modelBase.NewQueryOptions()
	options.Filter.Equal("status", scheduleModel.InstanceStatusActive)
	instances, err := legacyList[*scheduleModel.ActivityInstance](ctx, s.instances, options)
	if err != nil {
		return result, fmt.Errorf("load active activity instances: %w", err)
	}
	for _, instance := range instances {
		result.Checked++
		_ = s.completeIfDueIsolated(ctx, instance, now, grace, result)
	}
	return result, nil
}

// completeIfDueIsolated rolls back only this completion when the scheduler
// already runs in a tenant transaction: nested transactions map to
// savepoints, so a failed block does not abort the batch.
func (s *instanceAutoEnd) completeIfDueIsolated(ctx context.Context, instance *scheduleModel.ActivityInstance, now time.Time, grace time.Duration, result *timetable.AutoEndResult) error {
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return s.completeIfDue(ctx, instance, now, grace, result)
	}
	return tenant.WithSavepoint(ctx, func(savepointCtx context.Context) error {
		return s.completeIfDue(savepointCtx, instance, now, grace, result)
	})
}

func (s *instanceAutoEnd) completeIfDue(ctx context.Context, instance *scheduleModel.ActivityInstance, now time.Time, grace time.Duration, result *timetable.AutoEndResult) error {
	if instance.Status != scheduleModel.InstanceStatusActive {
		result.SkippedNonActive++
		return nil
	}
	if instance.IsSpontaneous {
		result.SkippedSpontaneous++
		return nil
	}
	deadline := timetable.LifecycleBoundary(timezone.Date(instance.Date), instance.EndTime).Add(grace)
	if now.Before(deadline) {
		result.SkippedBeforeDeadline++
		return nil
	}
	if _, err := s.completion.Complete(ctx, instance.ID); err != nil {
		return s.classifyCompletionFailure(ctx, instance.ID, err, result)
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
		current, findErr := s.instances.FindByID(ctx, instanceID)
		if findErr != nil {
			if modelBase.IsNoRows(findErr) {
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
