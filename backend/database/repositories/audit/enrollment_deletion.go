package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/uptrace/bun"
)

type EnrollmentDeletionRepository struct {
	db *bun.DB
}

func NewEnrollmentDeletionRepository(db *bun.DB) auditModels.EnrollmentDeletionRepository {
	return &EnrollmentDeletionRepository{db: db}
}

func (r *EnrollmentDeletionRepository) Create(ctx context.Context, event *auditModels.EnrollmentDeletion) error {
	if event == nil {
		return fmt.Errorf("enrollment deletion audit event is required")
	}
	base.EnsureTenantID(ctx, event)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(event).
		ModelTableExpr(`audit.enrollment_deletions AS "enrollment_deletion"`).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create enrollment deletion audit event: %w", err)
	}
	return nil
}
