package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/uptrace/bun"
)

type TimeTrackingDeletionRepository struct {
	db *bun.DB
}

func NewTimeTrackingDeletionRepository(db *bun.DB) auditModels.TimeTrackingDeletionRepository {
	return &TimeTrackingDeletionRepository{db: db}
}

func (r *TimeTrackingDeletionRepository) Create(ctx context.Context, event *auditModels.TimeTrackingDeletion) error {
	if event == nil {
		return fmt.Errorf("time tracking deletion audit event is required")
	}
	if err := event.Validate(); err != nil {
		return fmt.Errorf("invalid time tracking deletion audit event: %w", err)
	}
	base.EnsureTenantID(ctx, event)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(event).
		ModelTableExpr(`audit.time_tracking_deletions AS "time_tracking_deletion"`).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create time tracking deletion audit event: %w", err)
	}
	return nil
}
