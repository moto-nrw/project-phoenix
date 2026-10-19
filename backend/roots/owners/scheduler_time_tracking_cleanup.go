package owners

import (
	"context"

	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// schedulerTimeTrackingCleanupPort binds the retained time-tracking cleanup to the
// scheduler's consumer-owned port. A nil service stays nil so the scheduler
// keeps skipping the task when no cleanup is wired.
func schedulerTimeTrackingCleanupPort(service workforceCompose.TimeTrackingCleanupService) schedulerPorts.TimeTrackingCleanupService {
	if service == nil {
		return nil
	}
	return schedulerTimeTrackingCleanup{service: service}
}

type schedulerTimeTrackingCleanup struct {
	service workforceCompose.TimeTrackingCleanupService
}

func (c schedulerTimeTrackingCleanup) CleanupExpiredTimeTrackingData(ctx context.Context) (*schedulerPorts.TimeTrackingCleanupResult, error) {
	result, err := c.service.CleanupExpiredTimeTrackingData(ctx)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return &schedulerPorts.TimeTrackingCleanupResult{
		SessionsDeleted: result.SessionsDeleted,
		AbsencesDeleted: result.AbsencesDeleted,
		StaffAffected:   result.StaffAffected,
		RetentionDays:   result.RetentionDays,
		DurationMS:      result.DurationMS,
	}, nil
}
