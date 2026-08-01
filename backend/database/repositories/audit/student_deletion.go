package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/uptrace/bun"
)

type StudentDeletionRepository struct {
	db *bun.DB
}

func NewStudentDeletionRepository(db *bun.DB) auditModels.StudentDeletionRepository {
	return &StudentDeletionRepository{db: db}
}

func (r *StudentDeletionRepository) Create(ctx context.Context, event *auditModels.StudentDeletion) error {
	if event == nil {
		return fmt.Errorf("student deletion audit event is required")
	}
	base.EnsureTenantID(ctx, event)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(event).
		ModelTableExpr(`audit.student_deletions AS "student_deletion"`).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create student deletion audit event: %w", err)
	}
	return nil
}
