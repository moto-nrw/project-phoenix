package audit

import (
	"context"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

type TimeTrackingDeletionRepository struct {
	runtime Runtime
}

func NewTimeTrackingDeletionRepository(runtime Runtime) auditModels.TimeTrackingDeletionRepository {
	return &TimeTrackingDeletionRepository{runtime: requireRuntime(runtime)}
}

func (r *TimeTrackingDeletionRepository) Create(ctx context.Context, event *auditModels.TimeTrackingDeletion) error {
	if event == nil {
		return fmt.Errorf("time tracking deletion audit event is required")
	}
	if err := event.Validate(); err != nil {
		return fmt.Errorf("invalid time tracking deletion audit event: %w", err)
	}
	return NewAppender(r.runtime).Append(ctx, event)
}
