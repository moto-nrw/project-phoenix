package schedule

import (
	"context"
	"errors"
	"fmt"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// AutoEndService completes due active care-plan instances through the same
// lifecycle service used by the manual completion endpoint.
type AutoEndService interface {
	RunForTenant(ctx context.Context, now time.Time, grace time.Duration) (*AutoEndResult, error)
}

// AutoEndResult summarizes one scheduler tick for one tenant.
type AutoEndResult struct {
	Checked               int
	Completed             int
	SkippedBeforeDeadline int
	SkippedSpontaneous    int
	SkippedNonActive      int
	SkippedConcurrent     int
	Failed                int
	DurationMS            int64
}

type autoEndService struct {
	instances  scheduleModel.ActivityInstanceRepository
	completion InstanceService
}

// NewAutoEndService creates the tenant-scoped automatic completion service.
func NewAutoEndService(instances scheduleModel.ActivityInstanceRepository, completion InstanceService) AutoEndService {
	if instances == nil {
		panic("schedule auto-end: ActivityInstanceRepository is required")
	}
	if completion == nil {
		panic("schedule auto-end: InstanceService is required")
	}
	return &autoEndService{instances: instances, completion: completion}
}

func (s *autoEndService) RunForTenant(ctx context.Context, now time.Time, grace time.Duration) (*AutoEndResult, error) {
	startedAt := time.Now()
	result := &AutoEndResult{}
	defer func() {
		result.DurationMS = time.Since(startedAt).Milliseconds()
	}()

	options := modelBase.NewQueryOptions()
	options.Filter.Equal("status", scheduleModel.InstanceStatusActive)
	instances, err := s.instances.List(ctx, options)
	if err != nil {
		return result, fmt.Errorf("load active activity instances: %w", err)
	}

	for _, instance := range instances {
		result.Checked++
		if err := s.completeIfDue(ctx, instance, now, grace, result); err != nil {
			return result, fmt.Errorf("auto-end instance %d: %w", instance.ID, err)
		}
	}

	return result, nil
}

func (s *autoEndService) completeIfDue(ctx context.Context, instance *scheduleModel.ActivityInstance, now time.Time, grace time.Duration, result *AutoEndResult) error {
	if instance.Status != scheduleModel.InstanceStatusActive {
		result.SkippedNonActive++
		return nil
	}
	if instance.IsSpontaneous {
		result.SkippedSpontaneous++
		return nil
	}
	deadline := instanceBoundary(instance.Date, instance.EndTime).Add(grace)
	if now.Before(deadline) {
		result.SkippedBeforeDeadline++
		return nil
	}
	if _, err := s.completion.Complete(ctx, instance.ID); err != nil {
		if errors.Is(err, ErrInstanceMoved) || errors.Is(err, ErrInstanceNotFound) {
			result.SkippedConcurrent++
			return nil
		}
		if errors.Is(err, ErrInvalidInstanceTransition) {
			current, findErr := s.instances.FindByID(ctx, instance.ID)
			if findErr != nil {
				if modelBase.IsNoRows(findErr) {
					result.SkippedConcurrent++
					return nil
				}
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
	result.Completed++
	return nil
}
