package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
	"github.com/moto-nrw/project-phoenix/services/scheduler"
)

// schedulerTimeTrackingCleanupPort binds the retained time-tracking cleanup to the
// scheduler's consumer-owned port. A nil service stays nil so the scheduler
// keeps skipping the task when no cleanup is wired.
func schedulerTimeTrackingCleanupPort(service timetracking.TimeTrackingCleanupService) scheduler.TimeTrackingCleanupService {
	if service == nil {
		return nil
	}
	return schedulerTimeTrackingCleanup{service: service}
}

type schedulerTimeTrackingCleanup struct {
	service timetracking.TimeTrackingCleanupService
}

func (c schedulerTimeTrackingCleanup) CleanupExpiredTimeTrackingData(ctx context.Context) (*scheduler.TimeTrackingCleanupResult, error) {
	result, err := c.service.CleanupExpiredTimeTrackingData(ctx)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return &scheduler.TimeTrackingCleanupResult{
		SessionsDeleted: result.SessionsDeleted,
		AbsencesDeleted: result.AbsencesDeleted,
		StaffAffected:   result.StaffAffected,
		RetentionDays:   result.RetentionDays,
		DurationMS:      result.DurationMS,
	}, nil
}
